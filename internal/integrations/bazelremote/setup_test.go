// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package bazelremote

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestClientKeyPair(t *testing.T) {
	privateKeyPEM, publicKeyPEM, err := clientKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	privateBlock, _ := pem.Decode(privateKeyPEM)
	if privateBlock == nil || privateBlock.Type != "PRIVATE KEY" {
		t.Fatalf("invalid private key PEM: %q", privateKeyPEM)
	}
	privateKey, err := x509.ParsePKCS8PrivateKey(privateBlock.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := privateKey.(*ecdsa.PrivateKey); !ok {
		t.Fatalf("private key has type %T, want *ecdsa.PrivateKey", privateKey)
	}
	publicBlock, _ := pem.Decode(publicKeyPEM)
	if publicBlock == nil || publicBlock.Type != "PUBLIC KEY" {
		t.Fatalf("invalid public key PEM: %q", publicKeyPEM)
	}
	if _, err := x509.ParsePKIXPublicKey(publicBlock.Bytes); err != nil {
		t.Fatal(err)
	}
}

func TestRetryableProvisioningError(t *testing.T) {
	for _, code := range []codes.Code{codes.Unavailable, codes.Aborted, codes.DeadlineExceeded} {
		if !retryableProvisioningError(status.Error(code, "retry")) {
			t.Errorf("code %s should be retryable", code)
		}
	}
	if retryableProvisioningError(errors.New("no retry")) {
		t.Error("ordinary errors should not be retryable")
	}
}
