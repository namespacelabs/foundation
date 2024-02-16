// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package teleport

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/gravitational/teleport/api/profile"
	"namespacelabs.dev/foundation/internal/certificates"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/universe/teleport/configuration"
)

type teleportAppCreds struct {
	endpoint string
	certFile string
	keyFile  string
}

func tshAppsLogin(ctx context.Context, app, appCertPath string) error {
	// First we check if there is already a valid certificate as `tbot apps login` is very slow (>3s).
	if err := tasks.Return0(ctx, tasks.Action("teleport.validate-certificate").Arg("certificate", appCertPath), func(ctx context.Context) error {
		valid, _, err := certificates.CertFileIsValidFor(appCertPath, loginMinValidityTTL)
		if err != nil {
			return err
		}

		if !valid {
			return errors.New("Teleport app certificate has expired or expires soon")
		}

		return nil
	}); err == nil {
		return nil
	}

	return tasks.Return0(ctx, tasks.Action("tsh.apps-login").Arg("app", app), func(ctx context.Context) error {
		c := exec.CommandContext(ctx, tshBin, "--add-keys-to-agent", "no", "apps", "login", app, "--ttl", appLoginTTLMins)
		if err := c.Run(); err != nil {
			return err
		}

		return nil
	})
}

func resolveTeleportAppCreds(conf *configuration.Teleport, app string) (*teleportAppCreds, error) {
	if app == "" {
		return nil, nil
	}

	if conf.GetUserProfile() != "" {
		profile, err := profile.FromDir("", conf.GetUserProfile())
		if err != nil {
			return nil, fnerrors.UsageError("Login with 'tsh login'", "Teleport profile is not found.")
		}

		return &teleportAppCreds{
			endpoint: fmt.Sprintf("%s.%s", app, profile.WebProxyAddr),
			certFile: profile.AppCertPath(app),
			keyFile:  profile.UserKeyPath(),
		}, nil

	}

	switch app {
	case conf.GetRegistryApp():
		if conf.GetRegistryCertsDir() == "" {
			return nil, fnerrors.BadInputError("'registry_certs_dir' must be configured")
		}

		certPath := filepath.Join(conf.GetRegistryCertsDir(), "tlscert")
		keyPath := filepath.Join(conf.GetRegistryCertsDir(), "key")
		return &teleportAppCreds{
			endpoint: fmt.Sprintf("%s.%s", app, conf.GetProxyUrl()),
			certFile: certPath,
			keyFile:  keyPath,
		}, nil

	case conf.GetEcrCredentialsProxyApp():
		if conf.GetEcrCredentialsProxyCertsDir() == "" {
			return nil, fnerrors.BadInputError("'ecr_proxy_certs_dir' must be configured")
		}
		certPath := filepath.Join(conf.GetEcrCredentialsProxyCertsDir(), "tlscert")
		keyPath := filepath.Join(conf.GetEcrCredentialsProxyCertsDir(), "key")
		return &teleportAppCreds{
			endpoint: fmt.Sprintf("%s.%s", app, conf.GetProxyUrl()),
			certFile: certPath,
			keyFile:  keyPath,
		}, nil
	default:
		return nil, fnerrors.BadInputError("unknown Telepor application %q", app)
	}
}
