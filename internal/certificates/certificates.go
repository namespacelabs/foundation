// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package certificates

import (
	"crypto/x509"
	"encoding/pem"
	"time"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

func CertIsValid(bundle []byte) (bool, time.Time, error) {
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

	return now.Add(30 * 24 * time.Hour).Before(cert.NotAfter), cert.NotAfter, nil
}
