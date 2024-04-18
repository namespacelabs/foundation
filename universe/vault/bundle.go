package vault

import (
	"encoding/json"
	"os"
)

const EnvKey = "TLS_BUNDLE"

type TlsBundle struct {
	PrivateKeyPem  string   `json:"private_key_pem"`
	CertificatePem string   `json:"certificate_pem"`
	CaChainPem     []string `json:"ca_chain_pem"`
}

func Parse(data []byte) (*TlsBundle, error) {
	bundle := TlsBundle{}
	return &bundle, json.Unmarshal(data, &bundle)
}

func ParseFromEnv() (*TlsBundle, error) {
	return Parse([]byte(os.Getenv(EnvKey)))
}

func (b TlsBundle) Encode() ([]byte, error) {
	return json.Marshal(b)
}
