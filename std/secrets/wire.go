// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package secrets

import (
	"context"
	"flag"
	"os"
	"path/filepath"
)

var (
	simpleSecretsBasepath = flag.String("simple_secrets_basepath", "secrets/", "Basepath of local secret definitions.")
)

func ProvideSecret(ctx context.Context, caller string, req *Secret) (*Value, error) {
	// XXX this will be changed.
	m, err := ProvideSecrets(ctx, caller, &Secrets{Secret: []*Secret{req}})
	if err != nil {
		return nil, err
	}

	return m[req.Name], nil
}

func ProvideSecrets(ctx context.Context, caller string, req *Secrets) (map[string]*Value, error) {
	m, err := ProvideSecretsFromFS(ctx, os.DirFS(*simpleSecretsBasepath), caller, req)
	if err != nil {
		return nil, err
	}
	for _, s := range m {
		if s.Path != "" {
			// Make sure that paths are absolute.
			s.Path = filepath.Join(*simpleSecretsBasepath, s.Path)
		}
	}
	return m, nil
}