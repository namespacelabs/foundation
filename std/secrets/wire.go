// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package secrets

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"namespacelabs.dev/foundation/std/go/core"
)

var (
	serverSecretsBasepath = flag.String("server_secrets_basepath", "", "Basepath of local secret definitions.")
)

func ProvideSecret(ctx context.Context, req *Secret) (*Value, error) {
	// TODO change secrets to handle scoped instantiation correctly
	caller := core.PathFromContext(ctx).Last()

	sdm, err := loadDevMap(os.DirFS(*serverSecretsBasepath))
	if err != nil {
		return nil, fmt.Errorf("%v: failed to provision secrets: %w", caller, err)
	}

	cfg := lookupConfig(sdm, string(caller))
	if cfg == nil {
		return nil, fmt.Errorf("%v: no secret configuration definition in map.textpb", caller)
	}

	for _, secret := range cfg.Secret {
		if secret.Name == req.Name {
			if secret.FromPath == "" {
				return nil, fmt.Errorf("%v: no path definition for secret %q", caller, secret.Name)
			}
			if !filepath.IsAbs(secret.FromPath) {
				return nil, fmt.Errorf("%v: %s: expected an absolute path", caller, secret.Name)
			}
			return &Value{Name: secret.Name, Path: secret.FromPath}, nil
		}
	}

	return nil, fmt.Errorf("%v: %s: no secret configuration", caller, req.Name)
}
