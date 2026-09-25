// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package postgres

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"testing"
	"time"

	"gotest.tools/assert"
	"namespacelabs.dev/foundation/framework/resources"
	postgrespb "namespacelabs.dev/foundation/library/database/postgres"
)

func TestMaxConnLifetimeJitterOverride(t *testing.T) {
	db, err := NewDatabaseFromConnectionUriWithOverrides(
		context.Background(),
		nil,
		"postgres://user:password@localhost/database",
		nil,
		"test",
		&ConfigOverrides{MaxConnLifetimeJitter: 10 * time.Minute},
	)
	assert.NilError(t, err)
	t.Cleanup(func() {
		db.PgxPool().Close()
		assert.NilError(t, db.Close())
	})

	assert.Equal(t, db.PgxPool().Config().MaxConnLifetimeJitter, 10*time.Minute)
}

func TestVerificationDisabledPreservesConnectionTLSMode(t *testing.T) {
	db, err := NewDatabaseFromConnectionUri(
		context.Background(),
		&postgrespb.DatabaseInstance{CaCert: "invalid CA that must be ignored while verification is disabled"},
		"postgres://user:password@database.internal/database?sslmode=require",
		nil,
		"test",
	)
	assert.NilError(t, err)
	t.Cleanup(func() {
		db.PgxPool().Close()
		assert.NilError(t, db.Close())
	})

	assert.Assert(t, db.PgxPool().Config().ConnConfig.TLSConfig.InsecureSkipVerify)
}

func TestVerifyServerCertificateDefault(t *testing.T) {
	disabled := false
	enabled := true

	assert.Equal(t, verifyServerCertificate(nil), false)
	assert.Equal(t, verifyServerCertificate(&disabled), false)
	assert.Equal(t, verifyServerCertificate(&enabled), true)
}

func TestCustomCAVerifiesTrustAndHostname(t *testing.T) {
	trustedCA, trustedKey, trustedPEM := makeCA(t, "trusted CA")
	trustedServer := makeServerCertificate(t, trustedCA, trustedKey, "database.internal")
	untrustedCA, untrustedKey, _ := makeCA(t, "untrusted CA")
	untrustedServer := makeServerCertificate(t, untrustedCA, untrustedKey, "database.internal")

	for _, test := range []struct {
		name        string
		host        string
		certificate tls.Certificate
		checkError  func(*testing.T, error)
	}{
		{
			name:        "trusted certificate and matching hostname",
			host:        "database.internal",
			certificate: trustedServer,
			checkError: func(t *testing.T, err error) {
				assert.NilError(t, err)
			},
		},
		{
			name:        "wrong hostname",
			host:        "wrong.internal",
			certificate: trustedServer,
			checkError: func(t *testing.T, err error) {
				var hostnameError x509.HostnameError
				assert.Assert(t, errors.As(err, &hostnameError), "expected hostname verification error, got %v", err)
			},
		},
		{
			name:        "untrusted certificate",
			host:        "database.internal",
			certificate: untrustedServer,
			checkError: func(t *testing.T, err error) {
				var unknownAuthorityError x509.UnknownAuthorityError
				assert.Assert(t, errors.As(err, &unknownAuthorityError), "expected unknown authority error, got %v", err)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, err := NewDatabaseFromConnectionUriWithOverrides(
				context.Background(),
				&postgrespb.DatabaseInstance{CaCert: string(trustedPEM)},
				"postgres://user:password@"+test.host+"/database?sslmode=require",
				nil,
				"test",
				&ConfigOverrides{VerifyServerCertificate: true},
			)
			assert.NilError(t, err)
			t.Cleanup(func() {
				db.PgxPool().Close()
				assert.NilError(t, db.Close())
			})

			test.checkError(t, handshake(db.PgxPool().Config().ConnConfig.TLSConfig, test.certificate))
		})
	}
}

func TestReplicaUsesReplicaCA(t *testing.T) {
	_, _, primaryPEM := makeCA(t, "primary CA")
	replicaCA, replicaKey, replicaPEM := makeCA(t, "replica CA")
	replicaServer := makeServerCertificate(t, replicaCA, replicaKey, "replica.internal")

	data, err := json.Marshal(map[string]any{
		"database": map[string]any{
			"name":                   "app",
			"connection_uri":         "postgres://user:password@primary.internal/app?sslmode=require",
			"ca_cert":                string(primaryPEM),
			"replica_connection_uri": "postgres://user:password@replica.internal/app?sslmode=require",
			"replica_ca_cert":        string(replicaPEM),
		},
	})
	assert.NilError(t, err)
	parsed, err := resources.ParseResourceData(data)
	assert.NilError(t, err)

	db, err := ConnectToReplicaResource(context.Background(), parsed, "database", nil, "test", &ConfigOverrides{
		VerifyServerCertificate: true,
	})
	assert.NilError(t, err)
	t.Cleanup(func() {
		db.PgxPool().Close()
		assert.NilError(t, db.Close())
	})

	config := db.PgxPool().Config().ConnConfig.Config
	assert.Equal(t, config.Host, "replica.internal")
	assert.NilError(t, handshake(config.TLSConfig, replicaServer))
}

func makeCA(t *testing.T, name string) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.NilError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	assert.NilError(t, err)
	certificate, err := x509.ParseCertificate(der)
	assert.NilError(t, err)

	return certificate, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func makeServerCertificate(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, host string) tls.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.NilError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: host},
		DNSNames:     []string{host},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	assert.NilError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	assert.NilError(t, err)
	certificate, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
	)
	assert.NilError(t, err)

	return certificate
}

func handshake(clientConfig *tls.Config, serverCertificate tls.Certificate) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()

	serverResult := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverResult <- err
			return
		}
		defer conn.Close()
		server := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{serverCertificate}})
		serverResult <- server.Handshake()
	}()

	client, err := tls.Dial("tcp", listener.Addr().String(), clientConfig)
	if client != nil {
		_ = client.Close()
	}
	<-serverResult
	return err
}
