// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package k3s

import (
	"context"
	"fmt"
	"io"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"namespacelabs.dev/foundation/framework/kubernetes/kubeclient"
	"namespacelabs.dev/foundation/framework/rpcerrors"
	"namespacelabs.dev/foundation/internal/build/registry"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/internal/ssh"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/universe/k3s/configuration"
)

var (
	clusterConfigType = cfg.DefineConfigType[*configuration.SshEndpoint]()
)

func Register() {
	client.RegisterConfigurationProvider("k3s-ssh", provideCluster)

	cfg.RegisterConfigurationProvider(func(remote *configuration.Remote) ([]proto.Message, error) {
		if remote.Endpoint == nil {
			return nil, fnerrors.BadInputError("remote must be specified")
		}

		if remote.Registry == "" {
			return nil, fnerrors.BadInputError("registry must be specified")
		}

		messages := []proto.Message{
			&client.HostEnv{Provider: "k3s-ssh"},
			&registry.Registry{
				Url: remote.Registry,
				Transport: &registry.RegistryTransport{
					Ssh: &registry.RegistryTransport_SSH{
						User:           remote.Endpoint.User,
						PrivateKeyPath: remote.Endpoint.PrivateKeyPath,
						SshAddr:        remote.Endpoint.Address,
					},
				},
			},
			remote.Endpoint,
		}

		return messages, nil
	})
}

func provideCluster(ctx context.Context, cfg cfg.Configuration) (client.ClusterConfiguration, error) {
	conf, ok := clusterConfigType.CheckGet(cfg)
	if !ok {
		return client.ClusterConfiguration{}, fnerrors.InternalError("missing configuration")
	}

	// XXX use ssh tunnel
	config, err := makeRemoteConfig(ctx, fmt.Sprintf("https://%s:6443", conf.Address), ssh.Endpoint{
		User:           conf.User,
		PrivateKeyPath: conf.PrivateKeyPath,
		Address:        conf.Address,
	})
	if err != nil {
		return client.ClusterConfiguration{}, err
	}

	return client.ClusterConfiguration{
		Config: *config,
	}, nil
}

func makeRemoteConfig(ctx context.Context, publicEndpoint string, endpoint ssh.Endpoint) (*api.Config, error) {
	return tasks.Return(ctx, tasks.Action("k3s.fetch-remote-config").Arg("endpoint", endpoint.Address), func(ctx context.Context) (*api.Config, error) {
		deferred, err := ssh.Establish(ctx, endpoint)
		if err != nil {
			return nil, err
		}

		conn, err := deferred.Dial()
		if err != nil {
			return nil, err
		}

		session, err := conn.NewSession()
		if err != nil {
			return nil, err
		}

		defer session.Close()

		r, err := session.StdoutPipe()
		if err != nil {
			return nil, err
		}

		if err := session.Run("cat /etc/rancher/k3s/k3s.yaml"); err != nil {
			return nil, err
		}

		contents, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}

		return decodeKubeConfig(publicEndpoint, contents)
	})
}

func decodeKubeConfig(endpoint string, kubeData []byte) (*api.Config, error) {
	cfg, err := clientcmd.Load(kubeData)
	if err != nil {
		return nil, err
	}

	cluster := cfg.Clusters["default"]
	user := cfg.AuthInfos["default"]

	if cluster == nil {
		return nil, rpcerrors.Errorf(codes.FailedPrecondition, "cluster data is missing")
	}

	if user == nil {
		return nil, rpcerrors.Errorf(codes.FailedPrecondition, "user data is missing")
	}

	return kubeclient.MakeApiConfig(&kubeclient.StaticConfig{
		EndpointAddress:          endpoint,
		CertificateAuthorityData: cluster.CertificateAuthorityData,
		ClientCertificateData:    user.ClientCertificateData,
		ClientKeyData:            user.ClientKeyData,
	}), nil
}
