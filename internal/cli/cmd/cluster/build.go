// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/types"
	"github.com/docker/distribution/reference"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/moby/buildkit/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"golang.org/x/sys/unix"
	buildkitfw "namespacelabs.dev/foundation/framework/build/buildkit"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/files"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/parsing/platform"
	"namespacelabs.dev/foundation/internal/providers/nscloud"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/runtime/docker"
	"namespacelabs.dev/foundation/internal/sdk/buildctl"
	"namespacelabs.dev/foundation/internal/sdk/host"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/go-ids"
)

const (
	nscrRegistryUsername = "token"
)

var (
	// preferredBuildPlatform is a mapping between supported platforms and preferable build cluster.
	preferredBuildPlatform = map[string]buildPlatform{
		"linux/arm64":  "arm64",
		"linux/arm/v5": "arm64",
		"linux/arm/v6": "arm64",
		"linux/arm/v7": "arm64",
		"linux/arm/v8": "arm64",
	}
)

func NewBuildkitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "buildkit",
		Short: "Buildkit-related functionality.",
	}

	cmd.AddCommand(newBuildctlCmd())
	cmd.AddCommand(newBuildkitProxy())

	return cmd
}

func newBuildctlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "buildctl -- ...",
		Short: "Run buildctl on the target build cluster.",
	}

	buildCluster := cmd.Flags().String("cluster", "", "Set the type of a the build cluster to use.")
	platform := cmd.Flags().String("platform", "amd64", "One of amd64 or arm64.")

	cmd.Flags().MarkDeprecated("cluster", "use --platform instead")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *buildCluster != "" && *platform != "" {
			return fnerrors.New("--cluster and --platform are exclusive")
		}

		buildctlBin, err := buildctl.EnsureSDK(ctx, host.HostPlatform())
		if err != nil {
			return fnerrors.New("failed to download buildctl: %w", err)
		}

		var plat buildPlatform
		if *platform != "" {
			p, err := parseBuildPlatform(*platform)
			if err != nil {
				return err
			}
			plat = p
		} else {
			if p, ok := compatClusterIDAsBuildPlatform(buildClusterOrDefault(*buildCluster)); ok {
				plat = p
			} else {
				return fnerrors.New("expected --cluster=build-cluster or build-cluster-arm64")
			}
		}

		p, err := runBuildProxyWithRegistry(ctx, plat, false)
		if err != nil {
			return err
		}

		defer p.Cleanup()

		return runBuildctl(ctx, buildctlBin, p, args...)
	})

	return cmd
}

func buildClusterOrDefault(bp string) string {
	if bp == "" {
		return buildCluster
	}
	return bp
}

func newBuildkitProxy() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proxy",
		Short: "Run a platform-specific buildkit proxy.",
		Args:  cobra.NoArgs,
	}

	sockPath := cmd.Flags().String("sock_path", "", "If specified listens on the specified path.")
	platform := cmd.Flags().String("platform", "amd64", "One of amd64, or arm64.")
	background := cmd.Flags().String("background", "", "If specified runs the proxy in the background, and writes the process PID to the specified path.")
	connectAtStartup := cmd.Flags().Bool("connect_at_startup", true, "If true, eagerly connects to the build cluster.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, _ []string) error {
		plat, err := parseBuildPlatform(*platform)
		if err != nil {
			return err
		}

		if *background != "" {
			if *sockPath == "" {
				return fnerrors.New("--background requires --sock_path")
			}

			if *connectAtStartup {
				// Make sure the cluster exists before going to the background.
				if _, err := ensureBuildCluster(ctx, plat); err != nil {
					return err
				}
			}

			cmd := exec.Command(os.Args[0], "buildkit", "proxy", "--sock_path", *sockPath, "--platform", string(plat), "--connect_at_startup", fmt.Sprintf("%v", *connectAtStartup))
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Foreground: false,
				Setsid:     true,
			}

			if err := cmd.Start(); err != nil {
				return err
			}

			pid := cmd.Process.Pid
			// Make sure the child process is not cleaned up on exit.
			if err := cmd.Process.Release(); err != nil {
				return err
			}

			return os.WriteFile(*background, []byte(fmt.Sprintf("%d", pid)), 0644)
		}

		bp, err := runBuildProxy(ctx, plat, *sockPath, *connectAtStartup)
		if err != nil {
			return err
		}

		fmt.Fprintf(console.Stderr(ctx), "Listening on %s\n", bp.socketPath)

		defer bp.Cleanup()

		return bp.Serve()
	})

	return cmd
}

func runBuildctl(ctx context.Context, buildctlBin buildctl.Buildctl, p *buildProxyWithRegistry, args ...string) error {
	cmdLine := []string{"--addr", "unix://" + p.BuildkitAddr}
	cmdLine = append(cmdLine, args...)

	fmt.Fprintf(console.Debug(ctx), "buildctl %s\n", strings.Join(cmdLine, " "))

	buildctl := exec.CommandContext(ctx, string(buildctlBin), cmdLine...)
	buildctl.Env = os.Environ()
	buildctl.Env = append(buildctl.Env, fmt.Sprintf("DOCKER_CONFIG="+p.DockerConfigDir))

	return localexec.RunInteractive(ctx, buildctl)
}

type buildProxyWithRegistry struct {
	BuildkitAddr    string
	DockerConfigDir string
	Cleanup         func() error
}

type buildPlatform string

func parseBuildPlatform(value string) (buildPlatform, error) {
	switch strings.ToLower(value) {
	case "amd64":
		return "amd64", nil

	case "arm64":
		return "arm64", nil
	}

	return "", fnerrors.New("invalid build platform %q", value)
}

func ensureBuildCluster(ctx context.Context, platform buildPlatform) (*api.CreateClusterResult, error) {
	response, err := api.EnsureBuildCluster(ctx, api.Endpoint, buildClusterOpts(platform))
	if err != nil {
		return nil, err
	}

	if err := waitUntilReady(ctx, response); err != nil {
		return nil, err
	}

	return response, nil
}

type buildProxy struct {
	socketPath string
	sink       tasks.ActionSink
	instance   *buildClusterInstance
	listener   net.Listener
	cleanup    func() error
}

type buildClusterInstance struct {
	platform buildPlatform

	mu            sync.Mutex
	previous      *api.CreateClusterResult
	cancelRefresh func()
}

func (bp *buildClusterInstance) NewConn(ctx context.Context) (net.Conn, error) {
	// This is not our usual play; we're doing a lot of work with the lock held.
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if bp.previous != nil && (bp.previous.BuildCluster == nil || bp.previous.BuildCluster.Resumable) {
		if _, err := api.EnsureCluster(ctx, api.Endpoint, bp.previous.ClusterId); err == nil {
			return bp.rawDial(ctx, bp.previous)
		}
	}

	response, err := api.EnsureBuildCluster(ctx, api.Endpoint, buildClusterOpts(bp.platform))
	if err != nil {
		return nil, err
	}

	if bp.cancelRefresh != nil {
		bp.cancelRefresh()
		bp.cancelRefresh = nil
	}

	if bp.previous == nil || bp.previous.ClusterId != response.ClusterId {
		if err := waitUntilReady(ctx, response); err != nil {
			fmt.Fprintf(console.Warnings(ctx), "Failed to wait for buildkit to become ready: %v\n", err)
		}
	}

	if response.BuildCluster != nil && !response.BuildCluster.DoesNotRequireRefresh {
		bp.cancelRefresh = api.StartBackgroundRefreshing(ctx, response.ClusterId)
	}

	bp.previous = response

	return bp.rawDial(ctx, response)
}

func (bp *buildClusterInstance) Cleanup() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if bp.cancelRefresh != nil {
		bp.cancelRefresh()
		bp.cancelRefresh = nil
	}

	return nil
}

func (bp *buildClusterInstance) rawDial(ctx context.Context, response *api.CreateClusterResult) (net.Conn, error) {
	buildkitSvc, err := resolveBuildkitService(response)
	if err != nil {
		return nil, err
	}

	return api.DialEndpoint(ctx, buildkitSvc.Endpoint)
}

func runBuildProxy(ctx context.Context, platform buildPlatform, socketPath string, connectAtStart bool) (*buildProxy, error) {
	bp := &buildClusterInstance{platform: platform}

	if connectAtStart {
		if _, err := bp.NewConn(ctx); err != nil {
			return nil, err
		}
	}

	var cleanup func() error
	if socketPath == "" {
		sockDir, err := dirs.CreateUserTempDir("", fmt.Sprintf("buildkit.%v", platform))
		if err != nil {
			return nil, err
		}

		socketPath = filepath.Join(sockDir, "buildkit.sock")
		cleanup = func() error {
			return os.RemoveAll(sockDir)
		}
	} else {
		if err := unix.Unlink(socketPath); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	var d net.ListenConfig
	listener, err := d.Listen(ctx, "unix", socketPath)
	if err != nil {
		_ = bp.Cleanup()
		if cleanup != nil {
			_ = cleanup()
		}
		return nil, err
	}

	sink := tasks.SinkFrom(ctx)

	return &buildProxy{socketPath, sink, bp, listener, cleanup}, nil
}

func (bp *buildProxy) Cleanup() error {
	var errs []error
	errs = append(errs, bp.listener.Close())
	errs = append(errs, bp.instance.Cleanup())
	if bp.cleanup != nil {
		errs = append(errs, bp.cleanup())
	}
	return multierr.New(errs...)
}

func (bp *buildProxy) Serve() error {
	return serveProxy(bp.listener, func() (net.Conn, error) {
		return bp.instance.NewConn(tasks.WithSink(context.Background(), bp.sink))
	})
}

func runBuildProxyWithRegistry(ctx context.Context, platform buildPlatform, nscrOnlyRegistry bool) (*buildProxyWithRegistry, error) {
	p, err := runBuildProxy(ctx, platform, "", true)
	if err != nil {
		return nil, err
	}

	go func() {
		if err := p.Serve(); err != nil {
			log.Fatal(err)
		}
	}()

	newConfig := *configfile.New(config.ConfigFileName)

	if !nscrOnlyRegistry {
		// This is a special option to support calling `nsc build --name`, which
		// implies that they want to use nscr.io registry, without asking the user to
		// set the credentials earlier with `nsc docker-login`.
		// In that case we cannot copy the CredentialsStore from default config
		// because MacOS Docker Engine would ignore the cleartext credentials we injected.
		existing := config.LoadDefaultConfigFile(console.Stderr(ctx))

		// We don't copy over all authentication settings; only some.
		// XXX replace with custom buildctl invocation that merges auth in-memory.

		newConfig.AuthConfigs = existing.AuthConfigs
		newConfig.CredentialHelpers = existing.CredentialHelpers
		newConfig.CredentialsStore = existing.CredentialsStore
	}

	nsRegs, err := api.GetImageRegistry(ctx, api.Endpoint)
	if err != nil {
		return nil, err
	}

	token, err := fnapi.FetchTenantToken(ctx)
	if err != nil {
		return nil, err
	}
	for _, reg := range []*api.ImageRegistry{nsRegs.Registry, nsRegs.NSCR} {
		if reg != nil {
			newConfig.AuthConfigs[reg.EndpointAddress] = types.AuthConfig{
				ServerAddress: reg.EndpointAddress,
				Username:      nscrRegistryUsername,
				Password:      token.Raw(),
			}
		}
	}

	tmpDir := filepath.Dir(p.socketPath)
	credsFile := filepath.Join(tmpDir, config.ConfigFileName)
	if err := files.WriteJson(credsFile, newConfig, 0600); err != nil {
		p.Cleanup()
		return nil, err
	}

	return &buildProxyWithRegistry{p.socketPath, tmpDir, p.Cleanup}, nil
}

func resolveBuildkitService(response *api.CreateClusterResult) (*api.Cluster_ServiceState, error) {
	buildkitSvc := api.ClusterService(response.Cluster, "buildkit")
	if buildkitSvc == nil || buildkitSvc.Endpoint == "" {
		return nil, fnerrors.New("cluster is missing buildkit")
	}

	if buildkitSvc.Status != "READY" {
		return nil, fnerrors.New("expected buildkit to be READY, saw %q", buildkitSvc.Status)
	}

	return buildkitSvc, nil
}

func waitUntilReady(ctx context.Context, response *api.CreateClusterResult) error {
	buildkitSvc, err := resolveBuildkitService(response)
	if err != nil {
		return err
	}

	return tasks.Action("buildkit.wait-until-ready").Run(ctx, func(ctx context.Context) error {
		return buildkitfw.WaitReadiness(ctx, func() (*client.Client, error) {
			// We must fetch a token with our parent context, so we get a task sink etc.
			token, err := fnapi.FetchTenantToken(ctx)
			if err != nil {
				return nil, err
			}

			return client.New(ctx, response.ClusterId, client.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return api.DialEndpointWithToken(ctx, token, buildkitSvc.Endpoint)
			}))
		})
	})
}

func connectToBuildkit(ctx context.Context, response *api.CreateClusterResult) (net.Conn, error) {
	buildkitSvc, err := resolveBuildkitService(response)
	if err != nil {
		return nil, err
	}

	return api.DialEndpoint(ctx, buildkitSvc.Endpoint)
}

func NewBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build an image in a build cluster.",
		Args:  cobra.MaximumNArgs(1),
	}

	dockerFile := cmd.Flags().StringP("file", "f", "", "If set, specifies what Dockerfile to build.")
	push := cmd.Flags().BoolP("push", "p", false, "If specified, pushes the image to the target repository.")
	dockerLoad := cmd.Flags().Bool("load", false, "If specified, load the image to the local docker registry.")
	tags := cmd.Flags().StringSliceP("tag", "t", nil, "Attach the specified tags to the image.")
	platforms := cmd.Flags().StringSlice("platform", []string{}, "Set target platform for build.")
	buildArg := cmd.Flags().StringSlice("build-arg", nil, "Pass build arguments to the build.")
	names := cmd.Flags().StringSliceP("name", "n", nil, "Provide a list of name tags for the image in nscr.io Workspace registry")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, specifiedArgs []string) error {
		if len(*tags) > 0 && len(*names) > 0 {
			return fnerrors.New("usage of both --tag and --name flags is not supported")
		}

		// XXX: having multiple outputs is not supported by buildctl.
		if *push && *dockerLoad {
			return fnerrors.New("usage of both --push and --load flags is not supported")
		}

		if len(*platforms) > 1 && *dockerLoad {
			return fnerrors.New("multi-platform builds require --push, --load is not supported")
		}

		if len(*tags)+len(*names) == 0 && *push {
			return fnerrors.New("--push requires at least one tag or name")
		}

		if len(*platforms) == 0 {
			if *dockerLoad {
				*platforms = []string{platform.FormatPlatform(docker.HostPlatform())}
			} else {
				*platforms = []string{"linux/amd64"}
			}
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

		if len(*names) > 0 {
			// Either tags or names slice is set, but not both.
			// So, append all names in tags slice
			resp, err := api.GetImageRegistry(ctx, api.Endpoint)
			if err != nil {
				return fmt.Errorf("Could not fetch nscr.io repository: %w", err)
			}

			if resp.NSCR == nil {
				return fmt.Errorf("Could not fetch nscr.io repository")
			}

			for _, name := range *names {
				*tags = append(*tags, fmt.Sprintf("%s/%s/%s", resp.NSCR.EndpointAddress, resp.NSCR.Repository, name))
			}
		}

		parsedTags := make([]name.Tag, len(*tags))
		for k, tag := range *tags {
			parsed, err := name.NewTag(tag)
			if err != nil {
				return fmt.Errorf("invalid tag %s: %w", tag, err)
			}
			parsedTags[k] = parsed
		}

		// Allocate a list ahead of time to allow for concurrent access.
		completeFuncs := make([]func() error, len(*platforms))

		builders := map[buildPlatform]*buildProxyWithRegistry{}
		for _, p := range *platforms {
			resolved := determineBuildClusterPlatform(clusterProfiles.ClusterPlatform, p)
			if _, ok := builders[resolved]; !ok {
				buildProxy, err := runBuildProxyWithRegistry(ctx, resolved, len(*names) > 0)
				if err != nil {
					return err
				}
				defer buildProxy.Cleanup()

				builders[resolved] = buildProxy
			}
		}

		eg := executor.New(ctx, "nsc/build")

		images := make([]oci.RawImageWithPlatform, len(*platforms))
		for k, p := range *platforms {
			platformSpec, err := platform.ParsePlatform(p)
			if err != nil {
				return err
			}

			var imageNames []string

			// When performing a multi-platform build, we only need a single
			// remote reference to point an index at.
			if len(*platforms) > 1 && len(parsedTags) > 0 {
				imageNames = append(imageNames, fmt.Sprintf("%s:%s-%s", parsedTags[0].Repository.Name(), strings.ReplaceAll(platform.FormatPlatform(platformSpec), "/", "-"), ids.NewRandomBase32ID(4)))
			} else {
				for _, parsed := range parsedTags {
					imageNames = append(imageNames, parsed.Name())
				}
			}

			args := []string{
				"build",
				"--frontend=dockerfile.v0",
				"--local", "context=" + contextDir,
				"--local", "dockerfile=" + contextDir,
				"--opt", "platform=" + p,
			}

			if *dockerFile != "" {
				args = append(args, "--opt", "filename="+*dockerFile)
			}

			for _, arg := range *buildArg {
				args = append(args, "--opt", "build-arg:"+arg)
			}

			k := k // Capture k.
			switch {
			case *push:
				// buildctl parses output as csv; need to quote to pass commas to `name`.
				args = append(args, "--output", fmt.Sprintf("type=image,push=true,%q", "name="+strings.Join(imageNames, ",")))

				completeFuncs[k] = func() error {
					fmt.Fprintf(console.Stdout(ctx), "Pushed for %s:\n", p)
					for _, imageName := range imageNames {
						fmt.Fprintf(console.Stdout(ctx), "  %s\n", imageName)
					}

					ref, remoteOpts, err := oci.ParseRefAndKeychain(ctx, imageNames[0], oci.RegistryAccess{Keychain: nscloud.DefaultKeychain})
					if err != nil {
						return err
					}

					image, err := remote.Image(ref, remoteOpts...)
					if err != nil {
						return fnerrors.InvocationError("registry", "failed to fetch image: %w", err)
					}

					images[k] = oci.RawImageWithPlatform{
						Image:    image,
						Platform: platformSpec,
					}

					return nil
				}

			case *dockerLoad:
				// Load to local Docker registry
				f, err := os.CreateTemp("", "docker-image-nsc")
				if err != nil {
					return err
				}

				// We don't actually need it.
				f.Close()

				if len(imageNames) > 0 {
					// buildctl parses output as csv; need to quote to pass commas to `name`.
					args = append(args, "--output", fmt.Sprintf("type=docker,dest=%s,%q", f.Name(), "name="+strings.Join(imageNames, ",")))
				} else {
					args = append(args, "--output", fmt.Sprintf("type=docker,dest=%s", f.Name()))
				}

				completeFuncs[k] = func() error {
					defer os.Remove(f.Name())

					t := time.Now()
					dockerLoad := exec.CommandContext(ctx, "docker", "load", "-i", f.Name())
					if err := localexec.RunInteractive(ctx, dockerLoad); err != nil {
						return err
					}
					fmt.Fprintf(console.Stdout(ctx), "Took %v to upload the image to Docker.\n", time.Since(t))
					return nil
				}
			}

			eg.Go(func(ctx context.Context) error {
				return runBuildctl(ctx, buildctlBin, builders[determineBuildClusterPlatform(clusterProfiles.ClusterPlatform, p)], args...)
			})
		}

		if err := eg.Wait(); err != nil {
			return err
		}

		for _, fn := range completeFuncs {
			if fn != nil {
				if err := fn(); err != nil {
					return err
				}
			}
		}

		if len(images) > 1 && *push {
			index, err := oci.RawMakeIndex(images...)
			if err != nil {
				return err
			}

			fmt.Fprint(console.Stdout(ctx), "Multi-platform image:\n\n")

			for _, parsed := range parsedTags {
				if _, err := index.Push(ctx, oci.RepositoryWithAccess{
					Repository: parsed.Name(),
					RegistryAccess: oci.RegistryAccess{
						Keychain: nscloud.DefaultKeychain,
					},
				}, false); err != nil {
					return err
				}

				fmt.Fprintf(console.Stdout(ctx), "  %s\n", parsed.Name())
			}
		}

		if !*push {
			// On push, we already report what was built. Add a hint for other builds, too.
			fmt.Fprintf(console.Stdout(ctx), "\nBuilt %d images (platforms %s).\n", len(images), strings.Join(*platforms, ","))
		}

		return nil
	})

	return cmd
}

func determineBuildClusterPlatform(allowedClusterPlatforms []string, platform string) buildPlatform {
	// If requested platform is arm64 and "arm64" is allowed.
	if preferredBuildPlatform[platform] == "arm64" && slices.Contains(allowedClusterPlatforms, "arm64") {
		return "arm64"
	}

	return "amd64"
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
