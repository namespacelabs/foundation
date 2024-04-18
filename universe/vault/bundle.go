package vault

import (
	"crypto/tls"
	"crypto/x509"
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
	tb := TlsBundle{}
	return &tb, json.Unmarshal(data, &tb)
}

func ParseFromEnv() (*TlsBundle, error) {
	return Parse([]byte(os.Getenv(EnvKey)))
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

func (tb TlsBundle) ClientConfid() (*tls.Config, error) {
	cert, err := tb.Certificate()
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      tb.CAPool(),
	}, nil
}
