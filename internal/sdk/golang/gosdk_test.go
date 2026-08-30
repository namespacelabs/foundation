// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package golang

import (
	"testing"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestSDKReference(t *testing.T) {
	sdk, err := SDKReference("1.26", specs.Platform{OS: "linux", Architecture: "amd64"})
	if err != nil {
		t.Fatal(err)
	}

	if sdk.Version != "1.26.2" {
		t.Errorf("Version = %q, want %q", sdk.Version, "1.26.2")
	}
	if sdk.Ref.URL != "https://go.dev/dl/go1.26.2.linux-amd64.tar.gz" {
		t.Errorf("URL = %q, want %q", sdk.Ref.URL, "https://go.dev/dl/go1.26.2.linux-amd64.tar.gz")
	}
	if sdk.Ref.Digest.Algorithm != "sha256" {
		t.Errorf("digest algorithm = %q, want %q", sdk.Ref.Digest.Algorithm, "sha256")
	}
	if sdk.Ref.Digest.Hex != "990e6b4bbba816dc3ee129eaeaf4b42f17c2800b88a2166c265ac1a200262282" {
		t.Errorf("digest = %q, want pinned Go SDK digest", sdk.Ref.Digest.Hex)
	}
}
