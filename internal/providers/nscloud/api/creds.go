// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package api

import (
	"context"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
)

var DefaultKeychain oci.Keychain = defaultKeychain{}

type defaultKeychain struct{}

func (dk defaultKeychain) Resolve(ctx context.Context, r authn.Resource) (authn.Authenticator, error) {
	if strings.HasSuffix(r.RegistryStr(), ".nscluster.cloud") || strings.HasSuffix(r.RegistryStr(), ".namespace.systems") || r.RegistryStr() == "nscr.io" {
		return RegistryCreds(ctx)
	}

	return authn.Anonymous, nil
}
