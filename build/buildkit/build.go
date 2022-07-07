// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buildkit

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/session/secrets/secretsprovider"
	"github.com/moby/buildkit/session/sshforward/sshprovider"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/foundation/workspace/tasks"

	_ "github.com/moby/buildkit/client/connhelper/dockercontainer"
)

var BuildkitSecrets string

type clientInstance struct {
	conf *Overrides

	compute.DoScoped[*client.Client] // Only connect once per configuration.
}

func connectToClient(devHost *schema.DevHost, targetPlatform specs.Platform) compute.Computable[*client.Client] {
	conf := &Overrides{}

	devhost.ByBuildPlatform(targetPlatform).Select(devHost).Get(conf)

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

func MakeLocalState(src LocalContents, includePatterns ...string) llb.State {
	// Exlcluding files starting with ".". Consistent with dirs.IsExcluded
	excludePatterns := []string{"**/.*/"}
	for _, dir := range dirs.DirsToExclude {
		excludePatterns = append(excludePatterns, "**/"+dir+"/")
	}
	excludePatterns = append(excludePatterns, dirs.FilesToExclude...)
	excludePatterns = append(excludePatterns, devhost.HostOnlyFiles()...)
	if src.TemporaryIsWeb {
		// Not including the root tsconfig.json as it belongs to Node.js
		excludePatterns = append(excludePatterns, "tsconfig.json")
	}

	return llb.Local(src.Name(),
		llb.WithCustomName(fmt.Sprintf("Workspace %s (from %s)", src.Path, src.Module.ModuleName())),
		llb.SharedKeyHint(src.Name()),
		llb.LocalUniqueID(src.Name()),
		llb.ExcludePatterns(excludePatterns),
		llb.IncludePatterns(includePatterns))
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

func prepareSession(ctx context.Context) ([]session.Attachable, error) {
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
		authprovider.NewDockerAuthProvider(console.Stderr(ctx)),
		secretsprovider.NewSecretProvider(store),
	}

	// XXX make this configurable; eg at the devhost side.
	if os.Getenv("SSH_AUTH_SOCK") != "" {
		ssh, err := sshprovider.NewSSHAgentProvider([]sshprovider.AgentConfig{{ID: "default"}})
		if err != nil {
			return nil, err
		}

		attachables = append(attachables, ssh)
	}

	return attachables, nil
}
