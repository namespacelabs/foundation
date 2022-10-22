// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package client

import "k8s.io/client-go/tools/clientcmd/api"

func MakeApiConfig(cr *StaticConfig) *api.Config {
	cfg := api.NewConfig()
	cluster := api.NewCluster()
	cluster.CertificateAuthorityData = cr.CertificateAuthorityData
	cluster.Server = cr.EndpointAddress
	auth := api.NewAuthInfo()
	auth.ClientCertificateData = cr.ClientCertificateData
	auth.ClientKeyData = cr.ClientKeyData
	c := api.NewContext()
	c.Cluster = "default"
	c.AuthInfo = "default"

	cfg.Clusters["default"] = cluster
	cfg.AuthInfos["default"] = auth
	cfg.Contexts["default"] = c

	cfg.Kind = "Config"
	cfg.APIVersion = "v1"
	cfg.CurrentContext = "default"

	return cfg
}
