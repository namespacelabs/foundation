// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package private

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"

	instance "buf.build/gen/go/namespace/cloud/grpc/go/proto/namespace/private/instance/instancev1betagrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/metadata"
)

type InstanceServiceClient struct {
	instance.InstanceServiceClient
}

func MakeInstanceClient(ctx context.Context) (*InstanceServiceClient, error) {
	md, err := metadata.InstanceMetadataFromFile()
	if err != nil {
		return nil, err
	}

	tlsConfig, err := makeTLSConfigFromInstance(md)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.NewClient(md.InstanceEndpoint, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		return nil, err
	}

	cli := instance.NewInstanceServiceClient(conn)
	return &InstanceServiceClient{cli}, nil
}

func makeTLSConfigFromInstance(md metadata.InstanceMetadata) (*tls.Config, error) {
	caCert, err := os.ReadFile(md.Certs.HostPublicPemPath)
	if err != nil {
		return nil, fnerrors.Newf("could not ca open certificate file: %v", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fnerrors.Newf("failed to append ca certificate to pool: %v", err)
	}

	publicCert, err := os.ReadFile(md.Certs.PublicPemPath)
	if err != nil {
		return nil, fnerrors.Newf("could not public cert file: %v", err)
	}

	privateKey, err := os.ReadFile(md.Certs.PrivateKeyPath)
	if err != nil {
		return nil, fnerrors.Newf("could not private key file: %v", err)
	}

	keyPair, err := tls.X509KeyPair(publicCert, privateKey)
	if err != nil {
		return nil, fnerrors.Newf("could not load instance keys: %v", err)
	}

	return &tls.Config{
		RootCAs: caCertPool,
		GetClientCertificate: func(cri *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &keyPair, nil
		},
		// Instance certificates are not in the CA pool, so Go library will automatically
		// exclude them and client won't send its certificate, to force it to
		// send use GetClientCertificate instead
		// Certificates: []tls.Certificate{keyPair},
	}, nil
}
