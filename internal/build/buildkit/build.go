// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/docker/cli/cli/config"
	moby_buildkit_v1 "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/session/secrets/secretsprovider"
	"github.com/moby/buildkit/session/sshforward/sshprovider"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/mod/semver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"namespacelabs.dev/foundation/framework/rpcerrors"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"

	_ "github.com/moby/buildkit/client/connhelper/dockercontainer"
)

var (
	BuildkitSecrets string
	ForwardKeychain = false
)

const SSHAgentProviderID = "default"

type GatewayClient struct {
	*client.Client

	buildkitInDocker bool
	clientOpts       builtkitOpts
}

type builtkitOpts struct {
	SupportsOCILayoutInput  bool
	SupportsOCILayoutExport bool
	SupportsCanonicalBuilds bool
	HostPlatform            specs.Platform
}

func (cli *GatewayClient) UsesDocker() bool                                     { return cli.buildkitInDocker }
func (cli *GatewayClient) BuildkitOpts() builtkitOpts                           { return cli.clientOpts }
func (cli *GatewayClient) MakeClient(_ context.Context) (*GatewayClient, error) { return cli, nil }

type clientInstance struct {
	conf *Overrides

	compute.DoScoped[*GatewayClient] // Only connect once per configuration.
}

var OverridesConfigType = cfg.DefineConfigType[*Overrides]()

func Client(ctx context.Context, config cfg.Configuration, targetPlatform *specs.Platform) (*GatewayClient, error) {
	return compute.GetValue(ctx, MakeClient(config, targetPlatform))
}

func DeferClient(config cfg.Configuration, targetPlatform *specs.Platform) ClientFactory {
	return deferredMakeClient{config, targetPlatform}
}

type deferredMakeClient struct {
	config         cfg.Configuration
	targetPlatform *specs.Platform
}

func (d deferredMakeClient) MakeClient(ctx context.Context) (*GatewayClient, error) {
	return Client(ctx, d.config, d.targetPlatform)
}

func MakeClient(config cfg.Configuration, targetPlatform *specs.Platform) compute.Computable[*GatewayClient] {
	var conf *Overrides

	if targetPlatform != nil {
		conf, _ = OverridesConfigType.CheckGetForPlatform(config, *targetPlatform)
	} else {
		conf, _ = OverridesConfigType.CheckGet(config)
	}

	if conf.BuildkitAddr == "" && conf.HostedBuildCluster == nil && conf.ContainerName == "" {
		conf.ContainerName = DefaultContainerName
	}

	return &clientInstance{conf: conf}
}

var _ compute.Computable[*GatewayClient] = &clientInstance{}

func (c *clientInstance) Action() *tasks.ActionEvent {
	return tasks.Action("buildkit.connect")
}

func (c *clientInstance) Inputs() *compute.In {
	return compute.Inputs().Proto("conf", c.conf)
}

func (c *clientInstance) Compute(ctx context.Context, _ compute.Resolved) (*GatewayClient, error) {
	if c.conf.BuildkitAddr != "" {
		cli, err := client.New(ctx, c.conf.BuildkitAddr)
		if err != nil {
			return nil, err
		}

		return newClient(ctx, cli, false)
	}

	if c.conf.HostedBuildCluster != nil {
		fmt.Fprintf(console.Debug(ctx), "buildkit: connecting to nscloud %s/%d\n",
			c.conf.HostedBuildCluster.ClusterId, c.conf.HostedBuildCluster.TargetPort)

		cluster, err := api.GetCluster(ctx, api.Endpoint, c.conf.HostedBuildCluster.ClusterId)
		if err != nil {
			return nil, fnerrors.InternalError("failed to connect to buildkit in cluster: %w", err)
		}

		connect := func() (*client.Client, error) {
			return client.New(ctx, "buildkitd", client.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return api.DialPort(ctx, cluster.Cluster, int(c.conf.HostedBuildCluster.TargetPort))
			}))
		}

		if err := waitForBuildkit(ctx, connect); err != nil {
			return nil, err
		}

		cli, err := connect()
		if err != nil {
			return nil, err
		}

		return newClient(ctx, cli, false)
	}

	localAddr, err := EnsureBuildkitd(ctx, c.conf.ContainerName)
	if err != nil {
		return nil, err
	}

	cli, err := client.New(ctx, localAddr)
	if err != nil {
		return nil, err
	}

	// When disconnecting often get:
	//
	// WARN[0012] commandConn.CloseWrite: commandconn: failed to wait: signal: terminated
	//
	// compute.On(ctx).Cleanup(tasks.Action("buildkit.disconnect"), func(ctx context.Context) error {
	// 	return cli.Close()
	// })

	return newClient(ctx, cli, true)
}

func newClient(ctx context.Context, cli *client.Client, docker bool) (*GatewayClient, error) {
	var opts builtkitOpts

	workers, err := cli.ControlClient().ListWorkers(ctx, &moby_buildkit_v1.ListWorkersRequest{})
	if err != nil {
		return nil, fnerrors.InvocationError("buildkit", "failed to retrieve worker list: %w", err)
	}

	var hostPlatform *specs.Platform
	for _, x := range workers.Record {
		// We assume here that by convention the first platform is the host platform.
		if len(x.Platforms) > 0 {
			hostPlatform = &specs.Platform{
				Architecture: x.Platforms[0].Architecture,
				OS:           x.Platforms[0].OS,
			}
			break
		}
	}

	if hostPlatform == nil {
		return nil, fnerrors.InvocationError("buildkit", "no worker with platforms declared?")
	}

	opts.HostPlatform = *hostPlatform

	response, err := cli.ControlClient().Info(ctx, &moby_buildkit_v1.InfoRequest{})
	if err == nil {
		x, _ := json.MarshalIndent(response.GetBuildkitVersion(), "", "  ")
		fmt.Fprintf(console.Debug(ctx), "buildkit: version: %v\n", string(x))

		if semver.Compare(response.BuildkitVersion.GetVersion(), "v0.11.0-rc1") >= 0 {
			opts.SupportsOCILayoutInput = true
			opts.SupportsCanonicalBuilds = true
			opts.SupportsOCILayoutExport = false // Some casual testing seems to indicate that this is actually slower.
		}
	} else {
		fmt.Fprintf(console.Debug(ctx), "buildkit: Info failed: %v\n", err)
	}

	return &GatewayClient{Client: cli, buildkitInDocker: docker, clientOpts: opts}, nil
}

type FrontendRequest struct {
	Def            *llb.Definition
	OriginalState  *llb.State
	Frontend       string
	FrontendOpt    map[string]string
	FrontendInputs map[string]llb.State
	Secrets        []*schema.PackageRef
}

func MakeLocalExcludes(src LocalContents) []string {
	excludePatterns := []string{}
	excludePatterns = append(excludePatterns, dirs.BasePatternsToExclude...)
	excludePatterns = append(excludePatterns, devhost.HostOnlyFiles()...)
	excludePatterns = append(excludePatterns, src.ExcludePatterns...)

	return excludePatterns
}

func MakeLocalState(src LocalContents) llb.State {
	return llb.Local(src.Abs(),
		llb.WithCustomName(fmt.Sprintf("Workspace %s (from %s)", src.Path, src.Module.ModuleName())),
		llb.SharedKeyHint(src.Abs()),
		llb.LocalUniqueID(src.Abs()),
		llb.ExcludePatterns(MakeLocalExcludes(src)))
}

func prepareSession(ctx context.Context, keychain oci.Keychain, src runtime.GroundedSecrets, secrets []*schema.PackageRef) ([]session.Attachable, error) {
	var fs []secretsprovider.Source

	for _, def := range strings.Split(BuildkitSecrets, ";") {
		if def == "" {
			continue
		}

		parts := strings.Split(def, ":")
		if len(parts) != 3 {
			return nil, fnerrors.BadInputError("bad secret definition, expected {name}:env|file:{value}")
		}

		src := secretsprovider.Source{
			ID: parts[0],
		}

		switch parts[1] {
		case "env":
			src.Env = parts[2]
		case "file":
			src.FilePath = parts[2]
		default:
			return nil, fnerrors.BadInputError("expected env or file, got %q", parts[1])
		}

		fs = append(fs, src)
	}

	store, err := secretsprovider.NewStore(fs)
	if err != nil {
		return nil, err
	}

	secretValues := map[string][]byte{}
	if len(secrets) > 0 {
		if src == nil {
			return nil, fnerrors.InternalError("secrets specified, but secret source missing")
		}

		var errs []error
		for _, sec := range secrets {
			result, err := src.Get(ctx, sec)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if result.Value == nil {
				return nil, fnerrors.New("can't use secret %q, no value available (it's generated)", sec.Canonical())
			}

			secretValues[sec.Canonical()] = result.Value.Contents
		}

		if err := multierr.New(errs...); err != nil {
			return nil, err
		}
	}

	attachables := []session.Attachable{
		secretsprovider.NewSecretProvider(secretSource{store, secretValues}),
	}

	if ForwardKeychain {
		if keychain != nil {
			attachables = append(attachables, keychainWrapper{ctx: ctx, errorLogger: console.Output(ctx, "buildkit-auth"), keychain: keychain})
		}
	} else {
		dockerConfig := config.LoadDefaultConfigFile(console.Stderr(ctx))
		attachables = append(attachables, authprovider.NewDockerAuthProvider(dockerConfig))
	}

	// XXX make this configurable; eg at the devhost side.
	if os.Getenv("SSH_AUTH_SOCK") != "" {
		ssh, err := sshprovider.NewSSHAgentProvider([]sshprovider.AgentConfig{{ID: SSHAgentProviderID}})
		if err != nil {
			return nil, err
		}

		attachables = append(attachables, ssh)
	}

	return attachables, nil
}

type keychainWrapper struct {
	ctx         context.Context // Solve's parent context.
	errorLogger io.Writer
	keychain    oci.Keychain
}

func (kw keychainWrapper) Register(server *grpc.Server) {
	auth.RegisterAuthServer(server, kw)
}

func (kw keychainWrapper) Credentials(ctx context.Context, req *auth.CredentialsRequest) (*auth.CredentialsResponse, error) {
	response, err := kw.credentials(ctx, req.Host)

	if err == nil {
		fmt.Fprintf(console.Debug(kw.ctx), "[buildkit] AuthServer.Credentials %q --> %q\n", req.Host, response.Username)
	} else {
		fmt.Fprintf(console.Debug(kw.ctx), "[buildkit] AuthServer.Credentials %q: failed: %v\n", req.Host, err)

	}

	return response, err
}

func (kw keychainWrapper) credentials(ctx context.Context, host string) (*auth.CredentialsResponse, error) {
	// The parent context, not the incoming context is used, as the parent
	// context has an ActionSink attached (etc) while the incoming context is
	// managed by buildkit.
	authn, err := kw.keychain.Resolve(kw.ctx, resourceWrapper{host})
	if err != nil {
		return nil, err
	}

	if authn == nil {
		return &auth.CredentialsResponse{}, nil
	}

	authz, err := authn.Authorization()
	if err != nil {
		return nil, err
	}

	if authz.IdentityToken != "" || authz.RegistryToken != "" {
		fmt.Fprintf(kw.errorLogger, "%s: authentication type mismatch, got token expected username/secret", host)
		return nil, rpcerrors.Errorf(codes.InvalidArgument, "expected username/secret got token")
	}

	return &auth.CredentialsResponse{Username: authz.Username, Secret: authz.Password}, nil
}

func (kw keychainWrapper) FetchToken(ctx context.Context, req *auth.FetchTokenRequest) (*auth.FetchTokenResponse, error) {
	fmt.Fprintf(kw.errorLogger, "AuthServer.FetchToken %s\n", asJson(req))
	return nil, rpcerrors.Errorf(codes.Unimplemented, "unimplemented")
}

func (kw keychainWrapper) GetTokenAuthority(ctx context.Context, req *auth.GetTokenAuthorityRequest) (*auth.GetTokenAuthorityResponse, error) {
	fmt.Fprintf(kw.errorLogger, "AuthServer.GetTokenAuthority %s\n", asJson(req))
	return nil, rpcerrors.Errorf(codes.Unimplemented, "unimplemented")
}

func (kw keychainWrapper) VerifyTokenAuthority(ctx context.Context, req *auth.VerifyTokenAuthorityRequest) (*auth.VerifyTokenAuthorityResponse, error) {
	fmt.Fprintf(kw.errorLogger, "AuthServer.VerifyTokenAuthority %s\n", asJson(req))
	return nil, rpcerrors.Errorf(codes.Unimplemented, "unimplemented")
}

type resourceWrapper struct {
	host string
}

func (rw resourceWrapper) String() string      { return rw.host }
func (rw resourceWrapper) RegistryStr() string { return rw.host }

func asJson(msg any) string {
	str, _ := json.Marshal(msg)
	return string(str)
}
