// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkit

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/docker/cli/cli/config"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/session/secrets/secretsprovider"
	"github.com/moby/buildkit/session/sshforward/sshprovider"
	"namespacelabs.dev/foundation/framework/secrets"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build/buildkit/bkkeychain"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/schema"
)

type FrontendRequest struct {
	Def            *llb.Definition
	OriginalState  *llb.State
	Frontend       string
	FrontendAttrs  map[string]string
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

func prepareSession(ctx context.Context, keychain oci.Keychain, src secrets.GroundedSecrets, secrets []*schema.PackageRef) ([]session.Attachable, error) {
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

		eg := executor.New(ctx, "buildkit.load-secrets")

		results := make([][]byte, len(secrets))
		for k, sec := range secrets {
			k := k
			sec := sec

			eg.Go(func(ctx context.Context) error {
				result, err := src.Get(ctx, sec)
				if err != nil {
					return err
				}

				if result.Value == nil {
					return fnerrors.Newf("can't use secret %q, no value available (it's generated)", sec.Canonical())
				}

				results[k] = result.Value
				return nil
			})
		}

		if err := eg.Wait(); err != nil {
			return nil, err
		}

		for k, sec := range secrets {
			secretValues[sec.Canonical()] = results[k]
		}
	}

	attachables := []session.Attachable{
		secretsprovider.NewSecretProvider(secretSource{store, secretValues}),
	}

	if ForwardKeychain {
		if keychain != nil {
			attachables = append(attachables, bkkeychain.Wrapper{Context: ctx, ErrorLogger: console.Output(ctx, "buildkit-auth"), Keychain: keychain})
		}
	} else {
		dockerConfig := config.LoadDefaultConfigFile(console.Stderr(ctx))
		attachables = append(attachables, authprovider.NewDockerAuthProvider(dockerConfig, nil))
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
