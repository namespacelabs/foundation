// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package ctl

import (
	k8sapi "k8s.io/client-go/tools/clientcmd/api"
	"namespacelabs.dev/foundation/framework/kubernetes/kubeclient"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func MakeConfig(cluster *api.KubernetesCluster) k8sapi.Config {
	return *kubeclient.MakeApiConfig(&kubeclient.StaticConfig{
		EndpointAddress:          cluster.EndpointAddress,
		CertificateAuthorityData: cluster.CertificateAuthorityData,
		ClientCertificateData:    cluster.ClientCertificateData,
		ClientKeyData:            cluster.ClientKeyData,
	})
}
