// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buildkit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/session/secrets/secretsprovider"
	"github.com/moby/buildkit/session/sshforward/sshprovider"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"namespacelabs.dev/foundation/framework/rpcerrors"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/workspace/devhost"

	_ "github.com/moby/buildkit/client/connhelper/dockercontainer"
)

var (
	BuildkitSecrets string
	ForwardKeychain = false
)

const SSHAgentProviderID = "default"

type clientInstance struct {
	conf *Overrides

	compute.DoScoped[*client.Client] // Only connect once per configuration.
}

var OverridesConfigType = planning.DefineConfigType[*Overrides]()

func connectToClient(config planning.Configuration, targetPlatform specs.Platform) compute.Computable[*client.Client] {
	conf, _ := OverridesConfigType.CheckGetForPlatform(config, targetPlatform)

	if conf.BuildkitAddr == "" && conf.ContainerName == "" {
		conf.ContainerName = DefaultContainerName
	}

	return &clientInstance{conf: conf}
}

var _ compute.Computable[*client.Client] = &clientInstance{}

func (c *clientInstance) Action() *tasks.ActionEvent {
	return tasks.Action("buildkit.connect")
}

func (c *clientInstance) Inputs() *compute.In {
	return compute.Inputs().Proto("conf", c.conf)
}

func (c *clientInstance) Compute(ctx context.Context, _ compute.Resolved) (*client.Client, error) {
	conf, err := setupBuildkit(ctx, c.conf)
	if err != nil {
		return nil, err
	}

	cli, err := client.New(ctx, conf.Addr)
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

	return cli, nil
}

type frontendReq struct {
	Def            *llb.Definition
	Frontend       string
	FrontendOpt    map[string]string
	FrontendInputs map[string]llb.State
}

func MakeLocalState(src LocalContents) llb.State {
	return MakeCustomLocalState(src, MakeLocalStateOpts{})
}

type MakeLocalStateOpts struct {
	// If set, only files matching these patterns will be included in the state.
	Include []string
	// Added to the base exclude patterns. Override include patterns: if a file matches both, it is not included.
	Exclude []string
}

func MakeCustomLocalState(src LocalContents, opts MakeLocalStateOpts) llb.State {
	excludePatterns := []string{}
	excludePatterns = append(excludePatterns, dirs.BasePatternsToExclude...)
	excludePatterns = append(excludePatterns, devhost.HostOnlyFiles()...)
	excludePatterns = append(excludePatterns, opts.Exclude...)
	if src.TemporaryIsWeb {
		// Not including the root tsconfig.json as it belongs to Node.js
		excludePatterns = append(excludePatterns, "tsconfig.json")
	}

	return llb.Local(src.Name(),
		llb.WithCustomName(fmt.Sprintf("Workspace %s (from %s)", src.Path, src.Module.ModuleName())),
		llb.SharedKeyHint(src.Name()),
		llb.LocalUniqueID(src.Name()),
		llb.ExcludePatterns(excludePatterns),
		llb.IncludePatterns(opts.Include))
}

func makeDockerOpts(platforms []specs.Platform) map[string]string {
	return map[string]string{
		"platform": formatPlatforms(platforms),
	}
}

func formatPlatforms(ps []specs.Platform) string {
	strs := make([]string, len(ps))
	for k, p := range ps {
		strs[k] = devhost.FormatPlatform(p)
	}
	return strings.Join(strs, ",")
}

func prepareSession(ctx context.Context, keychain oci.Keychain) ([]session.Attachable, error) {
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

	attachables := []session.Attachable{
		secretsprovider.NewSecretProvider(store),
	}

	if ForwardKeychain {
		if keychain != nil {
			attachables = append(attachables, keychainWrapper{ctx: ctx, errorLogger: console.Output(ctx, "buildkit-auth"), keychain: keychain})
		}
	} else {
		attachables = append(attachables, authprovider.NewDockerAuthProvider(console.Stderr(ctx)))
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
