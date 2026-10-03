// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tlsbundle

import (
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
