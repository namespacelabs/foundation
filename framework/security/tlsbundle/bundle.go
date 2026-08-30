// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tlsbundle

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"os"
)

type TlsBundle struct {
	PrivateKeyPem  string   `json:"private_key_pem,omitempty"`
	CertificatePem string   `json:"certificate_pem,omitempty"`
	CaChainPem     []string `json:"ca_chain_pem,omitempty"`
}

func ParseTlsBundle(data []byte) (*TlsBundle, error) {
	tb := TlsBundle{}
	return &tb, json.Unmarshal(data, &tb)
}

func ParseTlsBundleFromEnv(key string) (*TlsBundle, error) {
	return ParseTlsBundle([]byte(os.Getenv(key)))
}

func (tb TlsBundle) Encode() ([]byte, error) {
	return json.Marshal(tb)
}

func (tb TlsBundle) CAPool() *x509.CertPool {
	pool := x509.NewCertPool()
	for _, cert := range tb.CaChainPem {
		pool.AppendCertsFromPEM([]byte(cert))
	}
	return pool
}

func (tb TlsBundle) Certificate() (tls.Certificate, error) {
	return tls.X509KeyPair([]byte(tb.CertificatePem), []byte(tb.PrivateKeyPem))
}

func (tb TlsBundle) ServerConfig() (*tls.Config, error) {
	cert, err := tb.Certificate()
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    tb.CAPool(),
	}, nil
}

func (tb TlsBundle) ClientConfig() (*tls.Config, error) {
	cert, err := tb.Certificate()
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      tb.CAPool(),
	}, nil
}
