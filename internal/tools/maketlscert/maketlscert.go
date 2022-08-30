// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package maketlscert

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/types"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func CreateCertificateChain(ctx context.Context, env *schema.Environment, r *types.TLSCertificateSpec) (*types.CertificateChain, error) {
	return tasks.Return(ctx, tasks.Action("certificate.create-bundle").Arg("key_size", keySize(env)), func(ctx context.Context) (*types.CertificateChain, error) {
		return createCertificateChain(env, r)
	})
}

func createCertificateChain(env *schema.Environment, r *types.TLSCertificateSpec) (*types.CertificateChain, error) {
	notBefore := time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC)
	notAfter := time.Date(2031, time.December, 31, 23, 59, 59, 59, time.UTC)

	caSerial, err := newSerialNumber()
	if err != nil {
		return nil, err
	}

	serverSerial, err := newSerialNumber()
	if err != nil {
		return nil, err
	}

	cacert := &x509.Certificate{
		SerialNumber: caSerial,
		Subject: pkix.Name{
			Organization: r.Organization,
			CommonName:   r.CommonNamePrefix + ", self-signed CA",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caPrivKey, err := rsa.GenerateKey(rand.Reader, keySize(env))
	if err != nil {
		return nil, err
	}

	caBundle, err := x509.CreateCertificate(rand.Reader, cacert, cacert, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, err
	}

	ca, err := pemEncode(caBundle, caPrivKey)
	if err != nil {
		return nil, err
	}

	cert := &x509.Certificate{
		SerialNumber: serverSerial,
		Subject: pkix.Name{
			Organization: r.Organization,
			CommonName:   r.CommonNamePrefix + ", Server",
		},
		NotBefore:   notBefore,
		NotAfter:    notAfter,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, keySize(env))
	if err != nil {
		return nil, err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, cacert, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, err
	}

	server, err := pemEncode(certBytes, certPrivKey)
	if err != nil {
		return nil, err
	}

	return &types.CertificateChain{CA: ca, Server: server}, nil
}

func pemEncode(bundle []byte, privKey *rsa.PrivateKey) (*types.Certificate, error) {
	// pem encode
	var caPEM bytes.Buffer
	if err := pem.Encode(&caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: bundle,
	}); err != nil {
		return nil, err
	}

	var caPrivKeyPEM bytes.Buffer
	if err := pem.Encode(&caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	}); err != nil {
		return nil, err
	}

	return &types.Certificate{PrivateKey: caPrivKeyPEM.Bytes(), Bundle: caPEM.Bytes()}, nil
}

func newSerialNumber() (*big.Int, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, serialNumberLimit)
}

func keySize(env *schema.Environment) int {
	if env.Purpose == schema.Environment_TESTING {
		// Speed up tests with cheaper keys.
		return 512
	}

	if env.Purpose == schema.Environment_PRODUCTION {
		return 4096
	}

	return 2048
}
