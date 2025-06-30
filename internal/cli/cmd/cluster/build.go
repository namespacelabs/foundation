// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	controllerapi "github.com/docker/buildx/controller/pb"
	"github.com/docker/buildx/util/buildflags"
	"github.com/docker/cli/cli/config"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/cmd/buildctl/build"
	"github.com/moby/buildkit/frontend/dockerui"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/util/progress/progresswriter"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc/codes"
	"namespacelabs.dev/foundation/framework/rpcerrors"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build/buildkit/bkkeychain"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/parsing/platform"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/runtime/docker"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/go-ids"
)

var (
	// preferredBuildPlatform is a mapping between supported platforms and preferable build cluster.
	preferredBuildPlatform = map[string]api.BuildPlatform{
		"linux/arm64":  "arm64",
		"linux/arm/v5": "arm64",
		"linux/arm/v6": "arm64",
		"linux/arm/v7": "arm64",
		"linux/arm/v8": "arm64",
	}
)

func NewBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build an image in a build instance.",
		Args:  cobra.MaximumNArgs(1),
	}

	dockerFile := cmd.Flags().StringP("file", "f", "", "If set, specifies what Dockerfile to build.")
	push := cmd.Flags().BoolP("push", "p", false, "If specified, pushes the image to the target repository.")
	dockerLoad := cmd.Flags().Bool("load", false, "If specified, load the image to the local docker registry.")
	tags := cmd.Flags().StringSliceP("tag", "t", nil, "Attach the specified tags to the image.")
	platforms := cmd.Flags().StringSlice("platform", []string{}, "Set target platform for build.")
	buildArg := cmd.Flags().StringSlice("build-arg", nil, "Pass build arguments to the build.")
	names := cmd.Flags().StringSliceP("name", "n", nil, "Provide a list of name tags for the image in nscr.io Workspace registry")
	secrets := cmd.Flags().StringArray("secret", nil, `Secret to expose to the build (format: "id=mysecret[,src=/local/secret]")`)
	useServerSideProxy := cmd.Flags().Bool("use_server_side_proxy", true, "If set, client is setup to use transparent mTLS server-side proxy instead of websockets.")
	_ = cmd.Flags().MarkHidden("use_server_side_proxy")

	outputLocal := cmd.Flags().String("output-local", "", "If set, outputs the build results to the specified directory.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, specifiedArgs []string) error {
		if len(*tags) > 0 && len(*names) > 0 {
			return fnerrors.Newf("usage of both --tag and --name flags is not supported")
		}

		// XXX: having multiple outputs is not supported by buildctl.
		var activeModes int
		if *push {
			activeModes++
		}
		if *dockerLoad {
			activeModes++
		}
		if *outputLocal != "" {
			activeModes++
		}
		if activeModes > 1 {
			return fnerrors.Newf("only one of --push, --load or --output-local can be used at a time")
		}

		if len(*platforms) > 1 && *dockerLoad {
			return fnerrors.Newf("multi-platform builds require --push, --load is not supported")
		}

		if len(*platforms) > 1 && *outputLocal != "" {
			return fnerrors.Newf("multi-platform builds require --push, --output-local is not supported")
		}

		if len(*tags)+len(*names) == 0 && *push {
			return fnerrors.Newf("--push requires at least one tag or name")
		}

		if len(*platforms) == 0 {
			if *dockerLoad {
				*platforms = []string{platform.FormatPlatform(docker.HostPlatform())}
			} else {
				*platforms = []string{"linux/amd64"}
			}
		}

		var parsedSecrets []*controllerapi.Secret
		if len(*secrets) > 0 {
			parsed, err := buildflags.ParseSecretSpecs(*secrets)
			if err != nil {
				return err
			}
			parsedSecrets = parsed
		}

		contextDir := "."
		if len(specifiedArgs) > 0 {
			contextDir = specifiedArgs[0]
		}

		if len(*names) > 0 {
			// Either tags or names slice is set, but not both.
			// So, append all names in tags slice
			resp, err := api.GetImageRegistry(ctx, api.Methods)
			if err != nil {
				return fmt.Errorf("Could not fetch nscr.io repository: %w", err)
			}

			if resp.NSCR == nil {
				return fmt.Errorf("Could not fetch nscr.io repository")
			}

			registryEpPrefix := fmt.Sprintf("%s/%s/", resp.NSCR.EndpointAddress, resp.NSCR.Repository)
			for _, name := range *names {
				if !strings.HasPrefix(name, registryEpPrefix) {
					name = registryEpPrefix + name
				}

				*tags = append(*tags, name)
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

		buildArgs, err := build.ParseOpt(*buildArg)
		if err != nil {
			return err
		}

		var fragments []BuildFragment
		var localImages []string
		for _, p := range *platforms {
			platformSpec, err := platform.ParsePlatform(p)
			if err != nil {
				return err
			}

			formatted := platform.FormatPlatform(platformSpec)

			var imageNames []string

			// When performing a multi-platform build, we only need a single
			// remote reference to point an index at.
			if len(*platforms) > 1 && len(parsedTags) > 0 {
				imageNames = append(imageNames, fmt.Sprintf("%s:%s-%s", parsedTags[0].Repository.Name(), strings.ReplaceAll(formatted, "/", "-"), ids.NewRandomBase32ID(4)))
			} else {
				for _, parsed := range parsedTags {
					imageNames = append(imageNames, parsed.Name())
				}
			}

			bf := BuildFragment{
				ContextDir: contextDir,
				Platform:   platformSpec,
				BuildArgs:  buildArgs,
				Secrets:    parsedSecrets,
			}

			if *dockerFile != "" {
				bf.Dockerfile = *dockerFile
			}

			switch {
			case *push:
				bf.Exports = append(bf.Exports, client.ExportEntry{
					Type: "image",
					Attrs: map[string]string{
						"push": "true",
						"name": strings.Join(imageNames, ","),
					},
				})

			case *dockerLoad:
				// Load to local Docker registry
				f, err := os.CreateTemp("", "docker-image-nsc")
				if err != nil {
					return err
				}

				defer os.Remove(f.Name())

				localImages = append(localImages, f.Name())

				export := client.ExportEntry{
					Type: "docker",
					Output: func(m map[string]string) (io.WriteCloser, error) {
						return f, nil
					},
					Attrs: map[string]string{},
				}

				if len(imageNames) > 0 {
					export.Attrs["name"] = strings.Join(imageNames, ",")
				}

				bf.Exports = append(bf.Exports, export)

			case *outputLocal != "":
				if err := os.MkdirAll(*outputLocal, 0755); err != nil {
					return err
				}

				bf.Exports = append(bf.Exports, client.ExportEntry{
					Type:      "local",
					OutputDir: *outputLocal,
				})
			}

			fragments = append(fragments, bf)
		}

		results, err := StartBuilds(ctx, fragments, MakeWireBuilder(*useServerSideProxy))
		if err != nil {
			return err
		}

		switch {
		case *push:
			images := make([]oci.RawImageWithPlatform, len(*platforms))

			for k, r := range results {
				name := r.ExporterResponse["image.name"]
				if name == "" {
					return rpcerrors.Errorf(codes.Internal, "expected image.name in result")
				}

				ref, remoteOpts, err := oci.ParseRefAndKeychain(ctx, name, oci.RegistryAccess{Keychain: keychain{}})
				if err != nil {
					return err
				}

				image, err := remote.Image(ref, remoteOpts...)
				if err != nil {
					return fnerrors.InvocationError("registry", "failed to fetch image: %w", err)
				}

				images[k] = oci.RawImageWithPlatform{
					Image:    image,
					Platform: fragments[k].Platform,
				}
			}

			if len(images) > 1 {
				index, err := oci.RawMakeIndex(images...)
				if err != nil {
					return err
				}

				fmt.Fprint(console.Stdout(ctx), "Multi-platform image:\n\n")

				for _, parsed := range parsedTags {
					if _, err := index.Push(ctx, oci.RepositoryWithAccess{
						Repository: parsed.Name(),
						RegistryAccess: oci.RegistryAccess{
							Keychain: keychain{},
						},
					}, false); err != nil {
						return err
					}

					fmt.Fprintf(console.Stdout(ctx), "  %s\n", parsed.Name())
				}
			} else {
				fmt.Fprint(console.Stdout(ctx), "Pushed:\n")
				for _, parsed := range parsedTags {
					fmt.Fprintf(console.Stdout(ctx), "  %s\n", parsed.Name())
				}
			}

		case *dockerLoad:
			for _, image := range localImages {
				t := time.Now()
				dockerLoad := exec.CommandContext(ctx, "docker", "load", "-i", image)
				if err := localexec.RunInteractive(ctx, dockerLoad); err != nil {
					return err
				}
				fmt.Fprintf(console.Stdout(ctx), "Took %v to upload the image to Docker.\n", time.Since(t))
			}
		}

		if !*push {
			// On push, we already report what was built. Add a hint for other builds, too.
			fmt.Fprintf(console.Stdout(ctx), "\nBuilt %d %s (platforms %s).\n", len(fragments), plural(len(fragments), "image", "images"), strings.Join(*platforms, ","))
		}

		return nil
	})

	return cmd
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}

	return plural
}

type BuildFragment struct {
	ContextDir         string
	Dockerfile         string
	DockerfileContents []byte
	BuildArgs          map[string]string
	Platform           specs.Platform
	Exports            []client.ExportEntry
	Secrets            []*controllerapi.Secret
}

func StartBuilds(ctx context.Context, fragments []BuildFragment, makeClient func(context.Context, specs.Platform) (*client.Client, error)) ([]*client.SolveResponse, error) {
	clients := make([]*client.Client, len(fragments))

	clientsEg := executor.New(ctx, "clients")

	for k, bf := range fragments {
		k := k   // Close k
		bf := bf // Close bf

		clientsEg.Go(func(ctx context.Context) error {
			cli, err := makeClient(ctx, bf.Platform)
			if err != nil {
				return err
			}

			clients[k] = cli
			return nil
		})
	}

	if err := clientsEg.Wait(); err != nil {
		return nil, err
	}

	done := console.EnterInputMode(ctx)
	defer done()

	// not using shared context to not disrupt display but let is finish reporting errors
	pw, err := progresswriter.NewPrinter(context.Background(), os.Stderr, "auto")
	if err != nil {
		return nil, err
	}

	mw := progresswriter.NewMultiWriter(pw)

	eg := executor.New(ctx, "nsc/build")

	results := make([]*client.SolveResponse, len(fragments))
	for k, bf := range fragments {
		k := k   // Close k
		bf := bf // Close bf

		startSingleBuild(eg, clients[k], mw, bf, func(sr *client.SolveResponse) error {
			results[k] = sr
			return nil
		})
	}

	eg.Go(func(ctx context.Context) error {
		select {
		case <-pw.Done():
			return pw.Err()

		case <-ctx.Done():
			return ctx.Err()
		}
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

func startSingleBuild(eg *executor.Executor, c *client.Client, mw *progresswriter.MultiWriter, bf BuildFragment, set func(*client.SolveResponse) error) {
	ref := identity.NewID()

	eg.Go(func(ctx context.Context) error {
		var attachable []session.Attachable

		dockerConfig := config.LoadDefaultConfigFile(console.Stderr(ctx))

		attachable = append(attachable, bkkeychain.Wrapper{
			Context:     ctx,
			ErrorLogger: io.Discard,
			Keychain:    keychain{},
			Fallback:    authprovider.NewDockerAuthProvider(dockerConfig, nil).(auth.AuthServer),
		})

		if len(bf.Secrets) > 0 {
			secrets, err := controllerapi.CreateSecrets(bf.Secrets)
			if err != nil {
				return err
			}
			attachable = append(attachable, secrets)
		}

		solveOpt := client.SolveOpt{
			Exports:  bf.Exports,
			Frontend: "dockerfile.v0",
			FrontendAttrs: map[string]string{
				"platform": platform.FormatPlatform(bf.Platform),
			},
			Session: attachable,
			Ref:     ref,
		}

		if bf.ContextDir != "" {
			solveOpt.LocalDirs = map[string]string{
				"context":    bf.ContextDir,
				"dockerfile": bf.ContextDir,
			}

			if bf.Dockerfile != "" {
				solveOpt.FrontendAttrs["filename"] = bf.Dockerfile
			}
		} else {
			if bf.Dockerfile != "" {
				return fmt.Errorf("dockerfile requires a contextdir")
			}
		}

		if bf.DockerfileContents != nil {
			solveOpt.FrontendInputs = map[string]llb.State{
				dockerui.DefaultLocalNameDockerfile: makeDockerfileState(bf.DockerfileContents),
				dockerui.DefaultLocalNameContext: llb.Scratch().
					File(llb.Mkfile("/empty", 0644, []byte{})), // Empty context.
			}
		}

		for k, v := range bf.BuildArgs {
			solveOpt.FrontendAttrs["build-arg:"+k] = v
		}

		var writers []progresswriter.Writer
		for _, at := range attachable {
			if s, ok := at.(interface {
				SetLogger(progresswriter.Logger)
			}); ok {
				w := mw.WithPrefix("", false)
				s.SetLogger(func(s *client.SolveStatus) {
					w.Status() <- s
				})
				writers = append(writers, w)
			}
		}

		defer func() {
			for _, w := range writers {
				close(w.Status())
			}
		}()

		statusCh := progresswriter.ResetTime(mw.WithPrefix(platform.FormatPlatform(bf.Platform), true)).Status()
		resp, err := c.Solve(ctx, nil, solveOpt, statusCh)
		if err != nil {
			return err
		}

		return set(resp)
	})
}

type keychain struct{}

func (dk keychain) Resolve(ctx context.Context, r authn.Resource) (authn.Authenticator, error) {
	if strings.HasSuffix(r.RegistryStr(), ".nscluster.cloud") || strings.HasSuffix(r.RegistryStr(), ".namespace.systems") || r.RegistryStr() == "nscr.io" || strings.HasSuffix(r.RegistryStr(), ".nscr.io") {
		return api.RegistryCreds(ctx)
	}

	return authn.DefaultKeychain.Resolve(r)
}

func determineBuildClusterPlatform(allowedClusterPlatforms []string, platform string) api.BuildPlatform {
	// If requested platform is arm64 and "arm64" is allowed.
	if preferredBuildPlatform[platform] == "arm64" && slices.Contains(allowedClusterPlatforms, "arm64") {
		return "arm64"
	}

	return "amd64"
}

func makeDockerfileState(contents []byte) llb.State {
	return llb.Scratch().
		File(llb.Mkfile("/Dockerfile", 0644, contents))
}

func MakeWireBuilder(useServerSideProxy bool) func(context.Context, specs.Platform) (*client.Client, error) {
	return func(ctx context.Context, p specs.Platform) (*client.Client, error) {
		sink := tasks.SinkFrom(ctx)

		bp, err := NewBuildCluster(ctx, platform.FormatPlatform(p), "")
		if err != nil {
			return nil, err
		}

		// Check if this is a remote build cluster that supports mTLS
		if useServerSideProxy {
			if remoteBp, ok := bp.(*RemoteBuildClusterInstance); ok {
				// Use mTLS connection for remote build clusters
				cli, err := createMTLSBuildKitClient(ctx, remoteBp)
				if err != nil {
					return nil, err
				}
				return cli, nil
			}
		}

		// Fallback to WebSocket connection for backward compatibility
		cli, err := client.New(ctx, "buildkitd", client.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			conn, _, err := bp.NewConn(tasks.WithSink(ctx, sink))
			return conn, err
		}))
		if err != nil {
			return nil, err
		}

		return cli, nil
	}
}

func createMTLSBuildKitClient(ctx context.Context, remoteBp *RemoteBuildClusterInstance) (*client.Client, error) {
	// Get builder configuration with certificates
	builderConfig, err := getBuilderConfigWithCerts(ctx, remoteBp.platform)
	if err != nil {
		return nil, fmt.Errorf("failed to get builder configuration: %w", err)
	}

	// Create BuildKit client with mTLS credentials
	cli, err := client.New(ctx, builderConfig.FullBuildkitEndpoint,
		client.WithCredentials(builderConfig.ClientCertPath, builderConfig.ClientKeyPath),
		client.WithServerConfig("", builderConfig.ServerCAPath))
	if err != nil {
		return nil, fmt.Errorf("failed to create BuildKit client with mTLS: %w", err)
	}

	return cli, nil
}

func getBuilderConfigWithCerts(ctx context.Context, platform api.BuildPlatform) (BuilderConfig, error) {
	// Get state directory for storing certificates
	stateDir, err := DetermineStateDir("", BuildkitProxyPath)
	if err != nil {
		return BuilderConfig{}, fmt.Errorf("failed to get state directory: %w", err)
	}

	builderConfigs, err := PrepareServerSideBuildxProxy(ctx, stateDir, []api.BuildPlatform{platform}, api.BuilderConfiguration{})
	if err != nil {
		return BuilderConfig{}, fmt.Errorf("failed to prepare certificates: %w", err)
	}

	if len(builderConfigs) == 0 {
		return BuilderConfig{}, fmt.Errorf("no builder configuration returned")
	}

	return builderConfigs[0], nil
}
