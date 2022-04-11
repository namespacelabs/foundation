// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package csrf

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
)

var protect mux.MiddlewareFunc

func Protect(h http.HandlerFunc) http.Handler {
	if protect == nil {
		panic("protect is nil, missing dependency on namespacelabs.dev/foundation/go/std/go/http/csrf?")
	}

	return protect(h)
}

func Prepare(ctx context.Context, deps *ExtensionDeps) error {
	key, err := base64.RawStdEncoding.DecodeString(string(deps.Token.MustValue()))
	if err != nil {
		return fmt.Errorf("failed to decode key: %v", err)
	}
	if len(key) != 32 {
		return fmt.Errorf("expected a key that is 32 bytes long, got %d", len(key))
	}

	protect = csrf.Protect(key)
	return nil
}
