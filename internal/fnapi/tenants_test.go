// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"encoding/json"
	"testing"
	"time"
)

func TestExchangeOIDCTokenRequestDuration(t *testing.T) {
	req := ExchangeOIDCTokenRequest{
		TenantId:  "tenant",
		OidcToken: "token",
		Duration:  protobufDuration(2*time.Hour + 500*time.Millisecond),
	}

	got, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	want := `{"tenant_id":"tenant","oidc_token":"token","duration":"7200.5s"}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestExchangeOIDCTokenRequestOmitsDefaultDuration(t *testing.T) {
	got, err := json.Marshal(ExchangeOIDCTokenRequest{TenantId: "tenant", OidcToken: "token"})
	if err != nil {
		t.Fatal(err)
	}

	want := `{"tenant_id":"tenant","oidc_token":"token"}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}
