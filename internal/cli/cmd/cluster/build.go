// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/types"
	manifeststore "github.com/docker/cli/cli/manifest/store"
	"github.com/docker/distribution/reference"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	registrytypes "github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/moby/buildkit/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	buildkitfw "namespacelabs.dev/foundation/framework/build/buildkit"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/files"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/parsing/platform"
	"namespacelabs.dev/foundation/internal/providers/nscloud"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/sdk/buildctl"
	"namespacelabs.dev/foundation/internal/sdk/host"
	"namespacelabs.dev/foundation/std/tasks"
)

var (
	// platformToCluster is a mapping between supported platforms and preferable build cluster.
	platformToCluster = map[string]string{
		"linux/amd64":    "amd64",
		"linux/386":      "amd64",
		"linux/mips64le": "amd64",
		"linux/ppc64le":  "amd64",
		"linux/riscv64":  "amd64",
		"linux/s390x":    "amd64",

		"linux/arm64":  "arm64",
		"linux/arm/v5": "arm64",
		"linux/arm/v6": "arm64",
		"linux/arm/v7": "arm64",
		"linux/arm/v8": "arm64",
	}
)

func NewBuildctlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "buildctl -- ...",
		Short: "Run buildctl on the target build cluster.",
	}

	buildCluster := cmd.Flags().String("cluster", buildCluster, "Set the type of a the build cluster to use.")
	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		buildctlBin, err := buildctl.EnsureSDK(ctx, host.HostPlatform())
		if err != nil {
			return fnerrors.New("failed to download buildctl: %w", err)
		}

		p, err := runBuildProxy(ctx, *buildCluster)
		if err != nil {
			return err
		}

		defer p.Cleanup()

		return runBuildctl(ctx, buildctlBin, p, args...)
	})

	return cmd
}

func runBuildctl(ctx context.Context, buildctlBin buildctl.Buildctl, p *buildProxy, args ...string) error {
	cmdLine := []string{"--addr", "unix://" + p.BuildkitAddr}
	cmdLine = append(cmdLine, args...)

	fmt.Fprintf(console.Debug(ctx), "buildctl %s\n", strings.Join(cmdLine, " "))

	buildctl := exec.CommandContext(ctx, string(buildctlBin), cmdLine...)
	buildctl.Env = os.Environ()
	buildctl.Env = append(buildctl.Env, fmt.Sprintf("DOCKER_CONFIG="+p.DockerConfigDir))

	return localexec.RunInteractive(ctx, buildctl)
}

type buildProxy struct {
	BuildkitAddr     string
	DockerConfigDir  string
	RegistryEndpoint string
	Repository       string
	Cleanup          func()
}

func runBuildProxy(ctx context.Context, cluster string) (*buildProxy, error) {
	response, err := api.EnsureBuildCluster(ctx, api.Endpoint, buildClusterOpts(cluster))
	if err != nil {
		return nil, err
	}

	if response.BuildCluster == nil || response.BuildCluster.Colocated == nil {
		return nil, fnerrors.New("cluster is not a build cluster")
	}

	if err := waitUntilReady(ctx, response); err != nil {
		return nil, err
	}

	p, err := runUnixSocketProxy(ctx, response.ClusterId, unixSockProxyOpts{
		Kind: "buildkit",
		Connect: func(ctx context.Context) (net.Conn, error) {
			return connect(ctx, response)
		},
	})
	if err != nil {
		return nil, err
	}

	t, err := api.RegistryCreds(ctx)
	if err != nil {
		p.Cleanup()
		return nil, err
	}

	existing := config.LoadDefaultConfigFile(console.Stderr(ctx))
	// We don't copy over all authentication settings; only some.
	// XXX replace with custom buildctl invocation that merges auth in-memory.
	newConfig := configfile.ConfigFile{
		AuthConfigs:       existing.AuthConfigs,
		CredentialHelpers: existing.CredentialHelpers,
		CredentialsStore:  existing.CredentialsStore,
	}

	newConfig.AuthConfigs[response.Registry.EndpointAddress] = types.AuthConfig{
		Username: t.Username,
		Password: t.Password,
	}

	credsFile := filepath.Join(p.TempDir, config.ConfigFileName)
	if err := files.WriteJson(credsFile, newConfig, 0600); err != nil {
		p.Cleanup()
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)

	go func() {
		_ = api.StartRefreshing(ctx, api.Endpoint, response.ClusterId, func(err error) error {
			fmt.Fprintf(console.Warnings(ctx), "Failed to refresh cluster: %v\n", err)
			return nil
		})
	}()

	return &buildProxy{p.SocketAddr, p.TempDir, response.Registry.EndpointAddress, response.Registry.Repository, func() {
		cancel()
		p.Cleanup()
	}}, nil
}

func waitUntilReady(ctx context.Context, response *api.CreateClusterResult) error {
	return tasks.Action("buildkit.wait-until-ready").Run(ctx, func(ctx context.Context) error {
		return buildkitfw.WaitReadiness(ctx, func() (*client.Client, error) {
			// We must fetch a token with our parent context, so we get a task sink etc.
			token, err := fnapi.FetchTenantToken(ctx)
			if err != nil {
				return nil, err
			}

			return client.New(ctx, response.ClusterId, client.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return api.DialPortWithToken(ctx, token, response.Cluster, int(response.BuildCluster.Colocated.TargetPort))
			}))
		})
	})
}

func serveBuildProxy(ctx context.Context, listener net.Listener, response *api.CreateClusterResult) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}

		go func() {
			defer conn.Close()

			peerConn, err := connect(ctx, response)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
				return
			}

			defer peerConn.Close()

			go func() {
				_, _ = io.Copy(conn, peerConn)
			}()

			_, _ = io.Copy(peerConn, conn)
		}()
	}
}

func connect(ctx context.Context, response *api.CreateClusterResult) (net.Conn, error) {
	return api.DialPort(ctx, response.Cluster, int(response.BuildCluster.Colocated.TargetPort))
}

func NewBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build an image in a build cluster.",
		Args:  cobra.MaximumNArgs(1),
	}

	dockerFile := cmd.Flags().StringP("file", "f", "", "If set, specifies what Dockerfile to build.")
	push := cmd.Flags().Bool("push", false, "If specified, pushes the image to the target repository.")
	dockerLoad := cmd.Flags().Bool("load", false, "If specified, load the image to the local docker registry.")
	tags := cmd.Flags().StringSliceP("tag", "t", nil, "Attach a tags to the image.")
	platforms := cmd.Flags().StringSlice("platform", []string{"linux/amd64"}, "Set target platform for build.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, specifiedArgs []string) error {
		// XXX: having multiple outputs is not supported by buildctl.
		if *push && *dockerLoad {
			return fnerrors.New("usage of both --push and --load flags is not supported")
		}

		if err := validateBuildPlatforms(*platforms); err != nil {
			return err
		}

		buildctlBin, err := buildctl.EnsureSDK(ctx, host.HostPlatform())
		if err != nil {
			return fnerrors.New("failed to download buildctl: %w", err)
		}

		clusterProfiles, err := api.GetProfile(ctx, api.Endpoint)
		if err != nil {
			return err
		}

		contextDir := "."
		if len(specifiedArgs) > 0 {
			contextDir = specifiedArgs[0]
		}

		reindexer := newImageReindexer(ctx)
		builtImages := make(map[string][]specs.Platform)

		errc := make(chan error, len(*platforms))
		var wg sync.WaitGroup
		for _, p := range *platforms {
			platformSpec, err := platform.ParsePlatform(p)
			if err != nil {
				return err
			}

			var imageNames []string
			for _, tag := range *tags {
				parsed, err := name.NewTag(tag)
				if err != nil {
					return fmt.Errorf("invalid tag %s: %w", tag, err)
				}

				builtImages[parsed.Name()] = append(builtImages[parsed.Name()], platformSpec)
				imageNames = append(imageNames, imageNameWithPlatform(parsed.Name(), platformSpec))
			}

			buildProxy, err := runBuildProxy(ctx, resolveBuildCluster(p, clusterProfiles.ClusterPlatform))
			if err != nil {
				return err
			}
			defer buildProxy.Cleanup()
			reindexer.setRegistryAccess(buildProxy.RegistryEndpoint, oci.RegistryAccess{Keychain: nscloud.DefaultKeychain})

			wg.Add(1)
			go func(platform string) {
				defer wg.Done()

				args := []string{
					"build",
					"--frontend=dockerfile.v0",
					"--local", "context=" + contextDir,
					"--local", "dockerfile=" + contextDir,
					"--opt", "platform=" + platform,
				}
				if *dockerFile != "" {
					args = append(args, "--opt", "filename="+*dockerFile)
				}

				var complete func() error
				switch {
				case *push:
					// buildctl parses output as csv; need to quote to pass commas to `name`.
					args = append(args, "--output",
						fmt.Sprintf("type=image,push=true,%q", "name="+strings.Join(imageNames, ",")))

					complete = func() error {
						fmt.Fprintf(console.Stdout(ctx), "Pushed:\n")
						for _, imageName := range imageNames {
							fmt.Fprintf(console.Stdout(ctx), "  %s\n", imageName)
						}
						return nil
					}
				case *dockerLoad:
					// Load to local Docker registry
					f, err := os.CreateTemp("", "docker-image-nsc")
					if err != nil {
						errc <- err
						return
					}

					defer os.Remove(f.Name())
					// We don't actually need it.
					f.Close()

					if len(imageNames) > 0 {
						// buildctl parses output as csv; need to quote to pass commas to `name`.
						args = append(args, "--output",
							fmt.Sprintf("type=docker,dest=%s,%q", f.Name(), "name="+strings.Join(imageNames, ",")))
					} else {
						args = append(args, "--output", fmt.Sprintf("type=docker,dest=%s", f.Name()))
					}

					complete = func() error {
						t := time.Now()
						dockerLoad := exec.CommandContext(ctx, "docker", "load", "-i", f.Name())
						if err := localexec.RunInteractive(ctx, dockerLoad); err != nil {
							return err
						}
						fmt.Fprintf(console.Stdout(ctx), "Took %v to upload the image to docker.\n", time.Since(t))
						return nil
					}
				}

				if err := runBuildctl(ctx, buildctlBin, buildProxy, args...); err != nil {
					errc <- err
					return
				}

				if err := complete(); err != nil {
					errc <- err
				}
			}(p)
		}

		wg.Wait()
		close(errc)

		var rErr error
		for err := range errc {
			if err != nil {
				rErr = errors.Join(rErr, err)
			}
		}
		if rErr != nil {
			return rErr
		}

		for imageName, platformSpecs := range builtImages {
			fmt.Fprintf(console.Stdout(ctx), "Reindex the image:\n %s\n", imageName)

			switch {
			case *dockerLoad:
				if err := reindexer.localReindex(ctx, imageName, platformSpecs); err != nil {
					return err
				}
			case *push:
				if err := reindexer.remoteReindex(ctx, imageName, platformSpecs); err != nil {
					return err
				}
			}
		}
		return nil
	})

	return cmd
}

type imageReindexer struct {
	manifestStore  manifeststore.Store
	registryAccess map[string]oci.RegistryAccess
}

func newImageReindexer(ctx context.Context) *imageReindexer {
	manifestStore := manifeststore.NewStore(filepath.Join(config.Dir(), "manifests"))
	return &imageReindexer{
		manifestStore:  manifestStore,
		registryAccess: make(map[string]oci.RegistryAccess),
	}
}

func (r imageReindexer) localReindex(ctx context.Context, imageName string, platformSpecs []specs.Platform) error {
	targetRef, err := normalizeReference(imageName)
	if err != nil {
		return err
	}

	_, err = r.manifestStore.GetList(targetRef)
	if err != nil && !manifeststore.IsNotFound(err) {
		return err
	}

	for _, p := range platformSpecs {
		pNamedRef, err := normalizeReference(imageNameWithPlatform(imageName, p))
		if err != nil {
			return err
		}

		manifests, err := r.manifestStore.Get(targetRef, pNamedRef)
		if err != nil {
			return err
		}

		if err := r.manifestStore.Save(targetRef, pNamedRef, manifests); err != nil {
			return fnerrors.New("failed to store image manifest: %w", err)
		}
	}

	return nil
}

func (r imageReindexer) remoteReindex(ctx context.Context, imageName string, platformSpecs []specs.Platform) error {
	imageRef, err := name.ParseReference(imageName)
	if err != nil {
		return err
	}

	registryAccess, ok := r.registryAccess[imageRef.Context().RegistryStr()]
	if !ok {
		registryAccess = oci.RegistryAccess{Keychain: registry.DefaultDockerKeychain}
	}

	remoteOpts, err := oci.RemoteOptsWithAuth(ctx, registryAccess, true)
	if err != nil {
		return err
	}

	adds := make([]mutate.IndexAddendum, 0, len(platformSpecs))
	for _, pSpec := range platformSpecs {
		imageIndex, err := r.pullImageManifest(ctx, imageNameWithPlatform(imageName, pSpec), pSpec, remoteOpts...)
		if err != nil {
			return err
		}

		if _, err := imageIndex.Digest(); err != nil {
			return err
		}

		mediaType, err := imageIndex.MediaType()
		if err != nil {
			return err
		}

		adds = append(adds, mutate.IndexAddendum{
			Add: imageIndex,
			Descriptor: v1.Descriptor{
				MediaType: mediaType,
				Platform: &v1.Platform{
					OS:           pSpec.OS,
					Architecture: pSpec.Architecture,
					Variant:      pSpec.Variant,
				},
			},
		})
	}

	imageIndex := mutate.AppendManifests(mutate.IndexMediaType(empty.Index, registrytypes.OCIImageIndex), adds...)

	// The Digest() is requested here to guarantee that the index can indeed be created.
	if _, err := imageIndex.Digest(); err != nil {
		return fnerrors.InternalError("failed to compute image index digest: %w", err)
	}

	return remote.WriteIndex(imageRef, imageIndex, remoteOpts...)
}

func (r imageReindexer) pullImageManifest(ctx context.Context, ref string, pSpec specs.Platform, opts ...remote.Option) (v1.Image, error) {
	namedRef, err := name.ParseReference(ref)
	if err != nil {
		return nil, err
	}

	remoteOpts := append(opts, remote.WithPlatform(v1.Platform{
		OS:           pSpec.OS,
		Architecture: pSpec.Architecture,
		Variant:      pSpec.Variant,
	}))
	imageManifest, err := remote.Image(namedRef, remoteOpts...)
	if err != nil {
		return nil, err
	}

	return imageManifest, nil
}

func (r imageReindexer) setRegistryAccess(registry string, access oci.RegistryAccess) {
	r.registryAccess[registry] = access
}

func validateBuildPlatforms(platforms []string) error {
	for _, p := range platforms {
		if _, ok := platformToCluster[p]; !ok {
			return fnerrors.New("platform is not supported: %s", p)
		}
	}
	return nil
}

func resolveBuildCluster(platform string, allowedClusterPlatforms []string) string {
	// If requested platform is arm64 and "arm64" is allowed.
	if platformToCluster[platform] == "arm64" && slices.Contains(allowedClusterPlatforms, "arm64") {
		return buildClusterArm64
	}

	return buildCluster
}

func imageNameWithPlatform(imageName string, platformSpec specs.Platform) string {
	return fmt.Sprintf("%s-%s", imageName, platformSpec.Architecture)
}

func normalizeReference(ref string) (reference.Named, error) {
	namedRef, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return nil, fnerrors.New("failed to parse image reference: %w", err)
	}
	if _, isDigested := namedRef.(reference.Canonical); !isDigested {
		return reference.TagNameOnly(namedRef), nil
	}
	return namedRef, nil
}
