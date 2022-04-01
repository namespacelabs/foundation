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
	serverSecretsBasepath = flag.String("server_secrets_basepath", "", "Basepath of local secret definitions.")
)

func ProvideSecret(ctx context.Context, caller string, req *Secret) (*Value, error) {
	m, err := ProvideSecretsFromFS(ctx, os.DirFS(*serverSecretsBasepath), caller, req)
	if err != nil {
		return nil, err
	}

	v := m[req.Name]
	if v.Path != "" && !filepath.IsAbs(v.Path) {
		v.Path = filepath.Join(*serverSecretsBasepath, v.Path)
	}

	return v, nil
}
