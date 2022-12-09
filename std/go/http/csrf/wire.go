// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package csrf

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
	"namespacelabs.dev/foundation/framework/resources"
)

var protect mux.MiddlewareFunc

func Protect(h http.HandlerFunc) http.Handler {
	if protect == nil {
		panic("protect is nil, missing dependency on namespacelabs.dev/foundation/go/std/go/http/csrf?")
	}

	return protect(h)
}

func Prepare(ctx context.Context) error {
	token, err := readSecretToken()
	if err != nil {
		return fmt.Errorf("failed to read secret token: %w", err)
	}
	key, err := base64.RawStdEncoding.DecodeString(string(token))
	if err != nil {
		return fmt.Errorf("failed to decode key: %v", err)
	}
	if len(key) != 32 {
		return fmt.Errorf("expected a key that is 32 bytes long, got %d", len(key))
	}

	protect = csrf.Protect(key)
	return nil
}

const secretTokenRef = "namespacelabs.dev/foundation/std/go/http/csrf:token-secret-resource"

func readSecretToken() ([]byte, error) {
	rs, err := resources.LoadResources()
	if err != nil {
		log.Fatal(err)
	}

	return resources.ReadSecret(rs, secretTokenRef)
}
