// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package minio

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"namespacelabs.dev/foundation/framework/resources"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

type EnsureBucketOptions struct {
	AccessKey       string
	SecretAccessKey string
	BucketName      string
	Endpoint        string
	Region          string
}

func EnsureBucket(ctx context.Context, instance EnsureBucketOptions) error {
	if instance.Endpoint == "" {
		return fnerrors.New("Endpoint must be set")
	}

	creds := credentials.NewStaticV4(instance.AccessKey, instance.SecretAccessKey, "")

	client, err := minio.New(instance.Endpoint, &minio.Options{
		Creds:        creds,
		Secure:       false,
		Region:       instance.Region,
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
	if err != nil {
		return fnerrors.New("failed to create client: %w", err)
	}

	if err := client.MakeBucket(ctx, instance.BucketName, minio.MakeBucketOptions{
		Region:        instance.Region,
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
