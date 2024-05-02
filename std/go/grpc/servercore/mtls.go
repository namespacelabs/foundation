// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package servercore

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"os"
)

type tlsBundle struct {
	PrivateKeyPem  string   `json:"private_key_pem"`
	CertificatePem string   `json:"certificate_pem"`
	CaChainPem     []string `json:"ca_chain_pem"`
}

func getMtlsConfig() (*tls.Config, error) {
	v := os.Getenv("FOUNDATION_GRPCSERVER_TLS_BUNDLE")
	if v == "" {
		return nil, nil
	}

	tb := tlsBundle{}
	if err := json.Unmarshal([]byte(v), &tb); err != nil {
		return nil, err
	}

	cert, err := tls.X509KeyPair([]byte(tb.CertificatePem), []byte(tb.PrivateKeyPem))
	if err != nil {
		return nil, err
	}

	pool := x509.NewCertPool()
	for _, cert := range tb.CaChainPem {
		pool.AppendCertsFromPEM([]byte(cert))
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
	}, nil
}
