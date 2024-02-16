// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package certificates

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"time"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

const oneMonthDuration = 30 * 24 * time.Hour

func CertFileIsValidFor(certFile string, forDuration time.Duration) (bool, time.Time, error) {
	b, err := os.ReadFile(certFile)
	if err != nil {
		return false, time.Time{}, err
	}

	return CertIsValidFor(b, forDuration)
}

func CertIsValidFor(bundle []byte, forDuration time.Duration) (bool, time.Time, error) {
	now := time.Now()

	// The rest is ignored, as we only care about the first pem block.
	block, _ := pem.Decode(bundle)
	if block == nil || block.Type != "CERTIFICATE" {
		return false, now, fnerrors.BadInputError("expected CERTIFICATE block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, now, fnerrors.BadInputError("invalid certificate")
	}

	return now.Add(forDuration).Before(cert.NotAfter), cert.NotAfter, nil
}

func CertIsValid(bundle []byte) (bool, time.Time, error) {
	return CertIsValidFor(bundle, oneMonthDuration)
}
