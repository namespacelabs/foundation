// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package gke

import (
	"context"
	"encoding/base64"
	"fmt"

	container "cloud.google.com/go/container/apiv1"
	"cloud.google.com/go/container/apiv1/containerpb"
	"google.golang.org/api/option"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/gcp"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/internal/sdk/gcloud"
	"namespacelabs.dev/foundation/std/cfg"
	gkepb "namespacelabs.dev/foundation/universe/gcp/gke"
)

var clusterConfigType = cfg.DefineConfigType[*gkepb.Cluster]()

func Register() {
	client.RegisterConfigurationProvider("gke", provideGKE)
	client.RegisterConfigurationProvider("gcp/gke", provideGKE)
}

func provideGKE(ctx context.Context, config cfg.Configuration) (client.ClusterConfiguration, error) {
	conf, ok := clusterConfigType.CheckGet(config)
	if !ok {
		return client.ClusterConfiguration{}, fnerrors.BadInputError("gke provider configured, but missing gke.Cluster")
	}

	project, ok := gcp.ProjectConfigType.CheckGet(config)
	if !ok {
		return client.ClusterConfiguration{}, fnerrors.BadInputError("gke provider configured, but missing gcp.Project")
	}

	ts := gcloud.NewTokenSource(ctx, project.Id)

	c, err := container.NewClusterManagerClient(ctx, option.WithTokenSource(ts))
	if err != nil {
		return client.ClusterConfiguration{}, err
	}

	allClusters, err := c.ListClusters(ctx, &containerpb.ListClustersRequest{
		Parent: fmt.Sprintf("projects/%s/locations/-", project.Id),
	})
	if err != nil {
		return client.ClusterConfiguration{}, err
	}

	var selected *containerpb.Cluster
	for _, cluster := range allClusters.Clusters {
		if cluster.Name == conf.Name {
			if selected != nil {
				return client.ClusterConfiguration{}, fnerrors.BadInputError("multiple clusters named %q", conf.Name)
			}

			selected = cluster
		}
	}

	if selected == nil {
		return client.ClusterConfiguration{}, fnerrors.BadInputError("no such cluster %q", conf.Name)
	}

	token, err := ts.Token()
	if err != nil {
		return client.ClusterConfiguration{}, err
	}

	x, err := Kubeconfig(selected, token.AccessToken)
	if err != nil {
		return client.ClusterConfiguration{}, err
	}

	return client.ClusterConfiguration{
		Config: *x,
		TokenProvider: func(ctx context.Context) (string, error) {
			token, err := ts.Token()
			if err != nil {
				return "", err
			}

			return token.AccessToken, nil
		},
	}, nil
}

func Kubeconfig(cluster *containerpb.Cluster, token string) (*clientcmdapi.Config, error) {
	if cluster.Endpoint == "" {
		return nil, fnerrors.BadInputError("cluster endpoint is missing")
	}

	if cluster.GetMasterAuth().GetClusterCaCertificate() == "" {
		return nil, fnerrors.BadInputError("cluster is missing a cluster ca certificate")
	}

	decoded, err := base64.StdEncoding.DecodeString(cluster.GetMasterAuth().GetClusterCaCertificate())
	if err != nil {
		return nil, fnerrors.BadInputError("failed to parse certificateauthority: %w", err)
	}

	clusterName := cluster.Name
	contextName := clusterName
	return &clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			clusterName: {
				Server:                   "https://" + cluster.Endpoint,
				CertificateAuthorityData: decoded,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			contextName: {
				Cluster:  clusterName,
				AuthInfo: contextName,
			},
		},
		CurrentContext: contextName,
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			contextName: {
				Token: token,
			},
		},
	}, nil
}
