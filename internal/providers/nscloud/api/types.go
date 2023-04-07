// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package api

import (
	"fmt"
	"time"
)

type CreateKubernetesClusterRequest struct {
	Ephemeral         bool     `json:"ephemeral,omitempty"`
	DocumentedPurpose string   `json:"documented_purpose,omitempty"`
	AuthorizedSshKeys []string `json:"authorized_ssh_keys,omitempty"`
	MachineType       string   `json:"machine_type,omitempty"`
	Feature           []string `json:"feature,omitempty"`
	UniqueTag         string   `json:"unique_tag,omitempty"`
}

type GetKubernetesClusterRequest struct {
	ClusterId string `json:"cluster_id,omitempty"`
}

type WaitKubernetesClusterRequest struct {
	ClusterId string `json:"cluster_id,omitempty"`
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

type CreateContainersRequest struct {
	MachineType string              `json:"machine_type,omitempty"`
	Container   []*ContainerRequest `json:"container,omitempty"`
	Compose     []*ComposeRequest   `json:"compose,omitempty"`
}

type StartContainersRequest struct {
	Id        string              `json:"id,omitempty"`
	Container []*ContainerRequest `json:"container,omitempty"`
}

type ContainerRequest struct {
	Name       string           `json:"name,omitempty"`
	Image      string           `json:"image,omitempty"`
	Args       []string         `json:"args,omitempty"`
	Flag       []string         `json:"flag,omitempty"`
	ExportPort []*ContainerPort `json:"export_port,omitempty"`
}

type ComposeRequest struct {
	Contents []byte `json:"contents,omitempty"`
}

type ContainerPort struct {
	Proto string `json:"proto,omitempty"`
	Port  int32  `json:"port,omitempty"`
}

type CreateContainersResponse struct {
	ClusterId  string       `json:"cluster_id,omitempty"`
	ClusterUrl string       `json:"cluster_url,omitempty"`
	Container  []*Container `json:"container,omitempty"`
}

type StartContainersResponse struct {
	Container []*Container `json:"container,omitempty"`
}

type Container struct {
	Id           string                             `json:"id,omitempty"`
	Name         string                             `json:"name,omitempty"`
	ExportedPort []*Container_ExportedContainerPort `json:"exported_port,omitempty"`
}

type Container_ExportedContainerPort struct {
	Proto       string `json:"proto,omitempty"`
	Port        int32  `json:"port,omitempty"`
	IngressFqdn string `json:"ingress_fqdn,omitempty"`
}

type ListKubernetesClustersRequest struct {
	IncludePreviousRuns bool   `json:"include_previous_runs,omitempty"`
	PaginationCursor    []byte `json:"pagination_cursor,omitempty"`
	MaxEntries          int64  `json:"max_entries,omitempty"`
}

type ListKubernetesClustersResponse struct {
	Clusters []KubernetesClusterMetadata `json:"cluster"`
}

type KubernetesClusterMetadata struct {
	ClusterId         string        `json:"cluster_id,omitempty"`
	Created           string        `json:"created,omitempty"`
	DestroyedAt       string        `json:"destroyed_at,omitempty"`
	Deadline          string        `json:"deadline,omitempty"`
	DocumentedPurpose string        `json:"documented_purpose,omitempty"`
	Shape             *ClusterShape `json:"shape,omitempty"`

	KubernetesDistribution string   `json:"kubernetes_distribution,omitempty"`
	Platform               []string `json:"platform,omitempty"`

	IngressDomain string `json:"ingress_domain,omitempty"`

	Label []*LabelEntry `json:"label,omitempty"`

	CreatorId      string              `json:"creator_id,omitempty"`
	GithubWorkflow *GithubWorkflowInfo `json:"github_workflow,omitempty"`
}

type KubernetesCluster struct {
	AppURL            string        `json:"app_url,omitempty"`
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

	CreatorId      string              `json:"creator_id,omitempty"`
	GithubWorkflow *GithubWorkflowInfo `json:"github_workflow,omitempty"`

	ServiceState []*Cluster_ServiceState `json:"service_state,omitempty"`
}

type Cluster_ServiceState struct {
	Name     string `json:"name,omitempty"`
	Status   string `json:"status,omitempty"`
	Endpoint string `json:"endpoint,omitempty"` // Service-specific endpoint.
	Public   bool   `json:"public,omitempty"`
}

type GithubWorkflowInfo struct {
	Repository string `json:"repository,omitempty"`
	RunId      string `json:"run_id,omitempty"`
	RunAttempt string `json:"run_attempt,omitempty"`
	Sha        string `json:"sha,omitempty"`
	Ref        string `json:"ref,omitempty"`
	Actor      string `json:"actor,omitempty"`
	Workflow   string `json:"workflow,omitempty"`
}

type GetImageRegistryResponse struct {
	Registry *ImageRegistry `json:"registry,omitempty"`
	NSCR     *ImageRegistry `json:"nscr,omitempty"`
}

type TailLogsRequest struct {
	ClusterID string          `json:"cluster_id,omitempty"`
	Include   []*LogsSelector `json:"include,omitempty"`
	Exclude   []*LogsSelector `json:"exclude,omitempty"`
}

type GetLogsRequest struct {
	ClusterID string          `json:"cluster_id,omitempty"`
	StartTs   *time.Time      `json:"start_ts,omitempty"`
	EndTs     *time.Time      `json:"end_ts,omitempty"`
	Include   []*LogsSelector `json:"include,omitempty"`
	Exclude   []*LogsSelector `json:"exclude,omitempty"`
}

type GetLogsResponse struct {
	LogBlock []LogBlock `json:"log_block,omitempty"`
}

type LogsSelector struct {
	Namespace string `json:"namespace,omitempty"`
	Pod       string `json:"pod,omitempty"`
	Container string `json:"container,omitempty"`
}

type LogBlock struct {
	Namespace string    `json:"namespace,omitempty"`
	Pod       string    `json:"pod,omitempty"`
	Container string    `json:"container,omitempty"`
	Line      []LogLine `json:"line,omitempty"`
}

type LogLine struct {
	Timestamp time.Time `json:"timestamp,omitempty"`
	Content   string    `json:"content,omitempty"`
	Stream    string    `json:"stream,omitempty"`
}

func (l LogLine) String() string {
	return fmt.Sprintf("%s stream=%s msg=%s", l.Timestamp.Format(time.RFC3339), l.Stream, l.Content)
}

type ImageRegistry struct {
	EndpointAddress string `json:"endpoint_address,omitempty"`
	Repository      string `json:"repository,omitempty"`
}

type ClusterShape struct {
	VirtualCpu      int32  `json:"virtual_cpu,omitempty"`
	MemoryMegabytes int32  `json:"memory_megabytes,omitempty"`
	MachineArch     string `json:"machine_arch,omitempty"`
}

type DestroyKubernetesClusterRequest struct {
	ClusterId string `json:"cluster_id,omitempty"`
}

type ReleaseKubernetesClusterRequest struct {
	ClusterId string `json:"cluster_id,omitempty"`
}

type WakeKubernetesClusterRequest struct {
	ClusterId string `json:"cluster_id,omitempty"`
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

type RefreshKubernetesClusterRequest struct {
	ClusterId string `json:"cluster_id,omitempty"`
}

type RefreshKubernetesClusterResponse struct {
	NewDeadline string `json:"new_deadline,omitempty"`
}

type GetKubernetesConfigRequest struct {
	ClusterId string `json:"cluster_id,omitempty"`
}

type GetKubernetesConfigResponse struct {
	Kubeconfig string `json:"kubeconfig,omitempty"`
}

type GetProfileResponse struct {
	ClusterPlatform []string `json:"cluster_platform,omitempty"`
}

type RegisterDefaultIngressRequest struct {
	ClusterId       string                  `json:"cluster_id,omitempty"`
	Prefix          string                  `json:"prefix,omitempty"`
	BackendEndpoint *IngressBackendEndpoint `json:"backend_endpoint,omitempty"`
}

type IngressBackendEndpoint struct {
	GuestIpAddr string `json:"guest_ip_addr,omitempty"`
	Port        int32  `json:"port,omitempty"`
}

type RegisterDefaultIngressResponse struct {
	Fqdn string `json:"fqdn,omitempty"`
}
