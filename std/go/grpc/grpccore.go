// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package grpc

import (
	"crypto/tls"

	"google.golang.org/grpc/credentials"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/go/core"
)

var ServerCreds credentials.TransportCredentials

func SetServerCredentials(creds credentials.TransportCredentials) {
	core.AssertNotRunning("grpc.SetServerCredentials")

	if ServerCreds != nil {
		panic("serverCreds were already set")
	}

	ServerCreds = creds
}

func SetServerOnlyTLS(sc *schema.Certificate) {
	cert, err := tls.X509KeyPair(sc.CertificateBundle, sc.PrivateKey)
	if err != nil {
		panic(err)
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.NoClientCert,
	}

	SetServerCredentials(credentials.NewTLS(config))
}
