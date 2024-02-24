// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package minio

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"google.golang.org/grpc/codes"
	"namespacelabs.dev/foundation/framework/resources"
	"namespacelabs.dev/foundation/framework/rpcerrors"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

type MinioOpts struct {
	AccessKey       string
	SecretAccessKey string
	Endpoint        string
	Region          string
	Secure          bool
	CaCert          []byte
}

type EnsureBucketOptions struct {
	BucketName string
	Minio      MinioOpts
}

func New(ctx context.Context, opts MinioOpts) (*minio.Client, error) {
	if opts.Endpoint == "" {
		return nil, rpcerrors.Errorf(codes.InvalidArgument, "Endpoint must be set")
	}

	var tlsConfig *tls.Config
	if opts.Secure {
		tlsConfig = &tls.Config{
			// Can't use SSLv3 because of POODLE and BEAST
			// Can't use TLSv1.0 because of POODLE and BEAST using CBC cipher
			// Can't use TLSv1.1 because of RC4 cipher usage
			MinVersion: tls.VersionTLS12,
		}

		if opts.CaCert != nil {
			caCertPool, err := x509.SystemCertPool()
			if err != nil {
				caCertPool = x509.NewCertPool()
			}

			caCertPool.AppendCertsFromPEM(opts.CaCert)
			tlsConfig.RootCAs = caCertPool
		}
	}

	creds := credentials.NewStaticV4(opts.AccessKey, opts.SecretAccessKey, "")

	return minio.New(opts.Endpoint, &minio.Options{
		Creds:        creds,
		Secure:       opts.Secure,
		Region:       opts.Region,
		BucketLookup: minio.BucketLookupPath,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				dialer := &net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 15 * time.Second,
				}

				conn, err := dialer.DialContext(ctx, network, addr)
				if err != nil {
					return nil, err
				}

				return conn, nil
			},
			MaxIdleConnsPerHost:   1024,
			WriteBufferSize:       32 << 10, // 32KiB moving up from 4KiB default
			ReadBufferSize:        32 << 10, // 32KiB moving up from 4KiB default
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			TLSClientConfig:       tlsConfig,
			ExpectContinueTimeout: 10 * time.Second,
			// Set this value so that the underlying transport round-tripper
			// doesn't try to auto decode the body of objects with
			// content-encoding set to `gzip`.
			//
			// Refer:
			//    https://golang.org/src/net/http/transport.go?h=roundTrip#L1843
			DisableCompression: true,
		},
	})
}

func EnsureBucket(ctx context.Context, instance EnsureBucketOptions) error {
	min, err := New(ctx, instance.Minio)
	if err != nil {
		return fnerrors.New("failed to create client: %w", err)
	}

	if err := min.MakeBucket(ctx, instance.BucketName, minio.MakeBucketOptions{
		Region:        instance.Minio.Region,
		ObjectLocking: false,
	}); err != nil {
		if merr, ok := err.(minio.ErrorResponse); ok {
			if merr.Code == "BucketAlreadyOwnedByYou" {
				return nil
			}
		}
		return fnerrors.New("failed to create bucket: %w", err)
	}

	return nil
}

func FromStatic(secretValue []byte) func() ([]byte, error) {
	return func() ([]byte, error) { return secretValue, nil }
}

func FromSecret(r *resources.Parsed, secretName string) func() ([]byte, error) {
	return func() ([]byte, error) {
		return resources.ReadSecret(r, secretName)
	}
}

type deferredFunc func() ([]byte, error)

type Endpoint struct {
	AccessKey          string
	SecretAccessKey    string
	PrivateEndpoint    string
	PrivateEndpointUrl string
	PublicEndpointUrl  string
}

func PrepareEndpoint(r *resources.Parsed, serverRef, serviceName string, accessKey, secretKey deferredFunc) (*Endpoint, error) {
	x, err := resources.LookupServerEndpoint(r, serverRef, serviceName)
	if err != nil {
		return nil, err
	}

	accessKeyID, err := accessKey()
	if err != nil {
		return nil, err
	}

	secretAccessKey, err := secretKey()
	if err != nil {
		return nil, err
	}

	ingress, err := resources.LookupServerFirstIngress(r, serverRef, serviceName)
	if err != nil {
		return nil, err
	}

	endpoint := &Endpoint{
		AccessKey:          string(accessKeyID),
		SecretAccessKey:    string(secretAccessKey),
		PrivateEndpoint:    x,
		PrivateEndpointUrl: fmt.Sprintf("http://%s", x),
	}

	if ingress != nil {
		endpoint.PublicEndpointUrl = *ingress
	}

	return endpoint, nil
}
