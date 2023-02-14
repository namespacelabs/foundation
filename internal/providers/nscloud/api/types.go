// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package api

type CreateKubernetesClusterRequest struct {
	OpaqueUserAuth    []byte       `json:"opaque_user_auth,omitempty"`
	Ephemeral         bool         `json:"ephemeral,omitempty"`
	DocumentedPurpose string       `json:"documented_purpose,omitempty"`
	AuthorizedSshKeys []string     `json:"authorized_ssh_keys,omitempty"`
	MachineType       string       `json:"machine_type,omitempty"`
	Feature           []string     `json:"feature,omitempty"`
	Attachment        []Attachment `json:"attachment,omitempty"`
}

type GetKubernetesClusterRequest struct {
	OpaqueUserAuth []byte `json:"opaque_user_auth,omitempty"`
	ClusterId      string `json:"cluster_id,omitempty"`
}

type WaitKubernetesClusterRequest struct {
	OpaqueUserAuth []byte `json:"opaque_user_auth,omitempty"`
	ClusterId      string `json:"cluster_id,omitempty"`
}

type CreateKubernetesClusterResponse struct {
	Status       string             `json:"status,omitempty"`
	ClusterId    string             `json:"cluster_id,omitempty"`
	Cluster      *KubernetesCluster `json:"cluster,omitempty"`
	Registry     *ImageRegistry     `json:"registry,omitempty"`
	BuildCluster *BuildCluster      `json:"build_cluster,omitempty"`
	Deadline     string             `json:"deadline,omitempty"`
}

type GetKubernetesClusterResponse struct {
	Cluster      *KubernetesCluster `json:"cluster,omitempty"`
	Registry     *ImageRegistry     `json:"registry,omitempty"`
	BuildCluster *BuildCluster      `json:"build_cluster,omitempty"`
	Deadline     string             `json:"deadline,omitempty"`
}

type StartCreateKubernetesClusterResponse struct {
	ClusterId       string             `json:"cluster_id,omitempty"`
	ClusterFragment *KubernetesCluster `json:"cluster_fragment,omitempty"`
	Registry        *ImageRegistry     `json:"registry,omitempty"`
	Deadline        string             `json:"deadline,omitempty"`
}

type ListKubernetesClustersRequest struct {
	OpaqueUserAuth      []byte `json:"opaque_user_auth,omitempty"`
	IncludePreviousRuns bool   `json:"include_previous_runs,omitempty"`
	PaginationCursor    []byte `json:"pagination_cursor,omitempty"`
	MaxEntries          int64  `json:"max_entries,omitempty"`
}

type KubernetesClusterList struct {
	Clusters []KubernetesCluster `json:"cluster"`
}

type KubernetesCluster struct {
	ClusterId         string        `json:"cluster_id,omitempty"`
	Created           string        `json:"created,omitempty"`
	DestroyedAt       string        `json:"destroyed_at,omitempty"`
	Deadline          string        `json:"deadline,omitempty"`
	SSHProxyEndpoint  string        `json:"ssh_proxy_endpoint,omitempty"`
	SshPrivateKey     []byte        `json:"ssh_private_key,omitempty"`
	DocumentedPurpose string        `json:"documented_purpose,omitempty"`
	Shape             *ClusterShape `json:"shape,omitempty"`

	EndpointAddress          string `json:"endpoint_address,omitempty"`
	CertificateAuthorityData []byte `json:"certificate_authority_data,omitempty"`
	ClientCertificateData    []byte `json:"client_certificate_data,omitempty"`
	ClientKeyData            []byte `json:"client_key_data,omitempty"`

	KubernetesDistribution string   `json:"kubernetes_distribution,omitempty"`
	Platform               []string `json:"platform,omitempty"`

	IngressDomain string `json:"ingress_domain,omitempty"`

	Label []*LabelEntry `json:"label,omitempty"`
}

type ImageRegistry struct {
	EndpointAddress string `json:"endpoint_address,omitempty"`
	Username        string `json:"username,omitempty"`
	Password        string `json:"password,omitempty"`
}

type ClusterShape struct {
	VirtualCpu      int32 `json:"virtual_cpu,omitempty"`
	MemoryMegabytes int32 `json:"memory_megabytes,omitempty"`
}

type DestroyKubernetesClusterRequest struct {
	OpaqueUserAuth []byte `json:"opaque_user_auth,omitempty"`
	ClusterId      string `json:"cluster_id,omitempty"`
}

type LabelEntry struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type BuildCluster struct {
	Colocated *BuildCluster_ColocatedPort `json:"colocated,omitempty"`
}

type BuildCluster_ColocatedPort struct {
	TargetPort int32  `json:"target_port,omitempty"`
	ClusterId  string `json:"cluster_id,omitempty"`
}

type Attachment struct {
	TypeURL string `json:"type_url,omitempty"`
	Content []byte `json:"content,omitempty"`
}
