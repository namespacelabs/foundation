// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/bcicen/jstream"
	"github.com/dustin/go-humanize"
	"github.com/spf13/pflag"
	"go.uber.org/atomic"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/environment"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/github"
	"namespacelabs.dev/foundation/std/tasks"
)

const AdminScope = "admin"

type API struct {
	StartCreateKubernetesCluster fnapi.Call[CreateKubernetesClusterRequest]
	GetKubernetesCluster         fnapi.Call[GetKubernetesClusterRequest]
	WaitKubernetesCluster        fnapi.Call[WaitKubernetesClusterRequest]
	ListKubernetesClusters       fnapi.Call[ListKubernetesClustersRequest]
	DestroyKubernetesCluster     fnapi.Call[DestroyKubernetesClusterRequest]
}

var Endpoint API

var (
	rpcEndpointOverride string
	regionName          string
)

func SetupFlags(flags *pflag.FlagSet, hide bool) {
	flags.StringVar(&rpcEndpointOverride, "nscloud_endpoint", "", "Where to dial to when reaching nscloud.")
	flags.StringVar(&regionName, "nscloud_region", "fra1", "Which region to use.")

	if hide {
		_ = flags.MarkHidden("nscloud_endpoint")
		_ = flags.MarkHidden("nscloud_region")
	}
}

func Register() {
	rpcEndpoint := rpcEndpointOverride
	if rpcEndpoint == "" {
		rpcEndpoint = fmt.Sprintf("https://api.%s.nscluster.cloud", regionName)
	}

	Endpoint = MakeAPI(rpcEndpoint)
}

func MakeAPI(endpoint string) API {
	fetchTenantToken := func(ctx context.Context) (string, error) {
		t, err := FetchTenantToken(ctx)
		if err != nil {
			return "", err
		}
		return t.Raw(), nil
	}

	return API{
		StartCreateKubernetesCluster: fnapi.Call[CreateKubernetesClusterRequest]{
			Endpoint:   endpoint,
			FetchToken: fetchTenantToken,
			Method:     "nsl.vm.api.VMService/StartCreateKubernetesCluster",
		},

		GetKubernetesCluster: fnapi.Call[GetKubernetesClusterRequest]{
			Endpoint:   endpoint,
			FetchToken: fetchTenantToken,
			Method:     "nsl.vm.api.VMService/GetKubernetesCluster",
		},

		WaitKubernetesCluster: fnapi.Call[WaitKubernetesClusterRequest]{
			Endpoint:   endpoint,
			FetchToken: fetchTenantToken,
			Method:     "nsl.vm.api.VMService/WaitKubernetesCluster",
		},

		ListKubernetesClusters: fnapi.Call[ListKubernetesClustersRequest]{
			Endpoint:   endpoint,
			FetchToken: fetchTenantToken,
			Method:     "nsl.vm.api.VMService/ListKubernetesClusters",
		},

		DestroyKubernetesCluster: fnapi.Call[DestroyKubernetesClusterRequest]{
			Endpoint:   endpoint,
			FetchToken: fetchTenantToken,
			Method:     "nsl.vm.api.VMService/DestroyKubernetesCluster",
		},
	}
}

func FetchTenantToken(ctx context.Context) (*auth.Token, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.fetch-tenant-token"), func(ctx context.Context) (*auth.Token, error) {
		if !fnapi.AdminMode {
			return auth.LoadTenantToken(ctx)
		}

		// In admin mode we exchange user token to a tenant token with `admin` scope.
		userToken, err := auth.GenerateToken(ctx)
		if err != nil {
			return nil, err
		}

		t, err := fnapi.ExchangeUserToken(ctx, userToken, AdminScope)
		if err != nil {
			return nil, err
		}

		return &auth.Token{TenantToken: t.TenantToken}, nil
	})
}

type CreateClusterResult struct {
	ClusterId    string
	Cluster      *KubernetesCluster
	Registry     *ImageRegistry
	BuildCluster *BuildCluster
	Deadline     *time.Time
}

type CreateClusterOpts struct {
	MachineType string
	Ephemeral   bool

	// Whether to keep the cluster alive, regardless of it being ephemeral.
	// This is typically needed if you want to execute multiple ns commands on an ephemeral cluster.
	KeepAlive bool

	Purpose           string
	Features          []string
	AuthorizedSshKeys []string
}

func CreateCluster(ctx context.Context, api API, opts CreateClusterOpts) (*StartCreateKubernetesClusterResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.cluster-create"), func(ctx context.Context) (*StartCreateKubernetesClusterResponse, error) {
		req := CreateKubernetesClusterRequest{
			Ephemeral:         opts.Ephemeral,
			DocumentedPurpose: opts.Purpose,
			MachineType:       opts.MachineType,
			Feature:           opts.Features,
			AuthorizedSshKeys: opts.AuthorizedSshKeys,
		}

		if github.IsRunningInActions() {
			attach, err := github.AttachmentFromEnv(ctx)
			if err != nil {
				return nil, err
			}

			content, err := json.Marshal(attach)
			if err != nil {
				return nil, err
			}

			req.Attachment = append(req.Attachment, Attachment{
				TypeURL: "namespacelabs.dev/foundation/schema.GithubAttachment",
				Content: content,
			})
		}

		if !environment.IsRunningInCI() {
			keys, err := UserSSHKeys()
			if err != nil {
				return nil, err
			}

			if keys != nil {
				actualKeys, err := compute.GetValue(ctx, keys)
				if err != nil {
					return nil, err
				}

				req.AuthorizedSshKeys = append(req.AuthorizedSshKeys, actualKeys...)
			}
		}

		var response StartCreateKubernetesClusterResponse
		if err := api.StartCreateKubernetesCluster.Do(ctx, req, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, fnerrors.New("failed to create cluster: %w", err)
		}

		tasks.Attachments(ctx).AddResult("cluster_id", response.ClusterId)

		if response.ClusterFragment != nil {
			if shape := response.ClusterFragment.Shape; shape != nil {
				tasks.Attachments(ctx).
					AddResult("cluster_cpu", shape.VirtualCpu).
					AddResult("cluster_ram", humanize.IBytes(uint64(shape.MemoryMegabytes)*humanize.MiByte))
			}
		}

		if opts.Ephemeral && !opts.KeepAlive {
			compute.On(ctx).Cleanup(tasks.Action("nscloud.cluster-cleanup"), func(ctx context.Context) error {
				if err := DestroyCluster(ctx, api, response.ClusterId); err != nil {
					// The cluster being gone is an acceptable state (it could have
					// been deleted by DeleteRecursively for example).
					if status.Code(err) == codes.NotFound {
						return nil
					}
				}

				return nil
			})
		}

		return &response, nil
	})
}

func CreateAndWaitCluster(ctx context.Context, api API, opts CreateClusterOpts) (*CreateClusterResult, error) {
	cluster, err := CreateCluster(ctx, api, opts)
	if err != nil {
		return nil, err
	}

	return WaitCluster(ctx, api, cluster.ClusterId)
}

func EnsureBuildCluster(ctx context.Context, api API) (*CreateClusterResult, error) {
	return CreateAndWaitCluster(ctx, api, CreateClusterOpts{MachineType: "16x32", Purpose: "build machine", Features: []string{"BUILD_CLUSTER"}})
}

func WaitCluster(ctx context.Context, api API, clusterId string) (*CreateClusterResult, error) {
	ctx, done := context.WithTimeout(ctx, 15*time.Minute) // Wait for cluster creation up to 15 minutes.
	defer done()

	var cr *CreateKubernetesClusterResponse
	if err := tasks.Action("nscloud.cluster-wait").Arg("cluster_id", clusterId).Run(ctx, func(ctx context.Context) error {
		var progress clusterCreateProgress
		progress.status.Store("CREATE_ACCEPTED_WAITING_FOR_ALLOCATION")
		tasks.Attachments(ctx).SetProgress(&progress)

		lastStatus := "<none>"
		for cr == nil {
			if err := ctx.Err(); err != nil {
				return err // Check if we've been cancelled.
			}

			// We continue to wait for the cluster to become ready until we observe a READY.
			if err := api.WaitKubernetesCluster.Do(ctx, WaitKubernetesClusterRequest{ClusterId: clusterId}, func(body io.Reader) error {
				decoder := jstream.NewDecoder(body, 1)

				// jstream gives us the streamed array segmentation, however it
				// returns map[string]interface{} rather than typed objects. We
				// re-triggering parsing into the response type so the remainder
				// of our codebase operates on types.

				for mv := range decoder.Stream() {
					var resp CreateKubernetesClusterResponse
					if err := reparse(mv.Value, &resp); err != nil {
						return fnerrors.InvocationError("nscloud", "failed to parse response: %w", err)
					}

					progress.set(resp.Status)
					lastStatus = resp.Status

					if resp.ClusterId != "" {
						clusterId = resp.ClusterId
					}

					if resp.Status == "READY" {
						cr = &resp
						return nil
					}
				}

				return fnerrors.InvocationError("nscloud", "stream closed before cluster became ready")
			}); err != nil {
				return fnerrors.InvocationError("nscloud", "cluster never became ready (last status was %q, cluster id: %s): %w", lastStatus, clusterId, err)
			}
		}

		tasks.Attachments(ctx).
			AddResult("cluster_address", cr.Cluster.EndpointAddress).
			AddResult("deadline", cr.Cluster.Deadline)

		return nil
	}); err != nil {
		return nil, err
	}

	result := &CreateClusterResult{
		ClusterId:    cr.ClusterId,
		Cluster:      cr.Cluster,
		Registry:     cr.Registry,
		BuildCluster: cr.BuildCluster,
	}

	if cr.Deadline != "" {
		t, err := time.Parse(time.RFC3339, cr.Deadline)
		if err == nil {
			result.Deadline = &t
		}
	}

	return result, nil
}

func DestroyCluster(ctx context.Context, api API, clusterId string) error {
	return api.DestroyKubernetesCluster.Do(ctx, DestroyKubernetesClusterRequest{
		ClusterId: clusterId,
	}, nil)
}

func GetCluster(ctx context.Context, api API, clusterId string) (*GetKubernetesClusterResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.get").Arg("id", clusterId), func(ctx context.Context) (*GetKubernetesClusterResponse, error) {
		var response GetKubernetesClusterResponse
		if err := api.GetKubernetesCluster.Do(ctx, GetKubernetesClusterRequest{ClusterId: clusterId}, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}
		return &response, nil
	})
}

func ListClusters(ctx context.Context, api API, previousRuns bool) (*KubernetesClusterList, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.cluster-list"), func(ctx context.Context) (*KubernetesClusterList, error) {
		var list KubernetesClusterList
		if err := api.ListKubernetesClusters.Do(ctx, ListKubernetesClustersRequest{
			IncludePreviousRuns: previousRuns,
		}, fnapi.DecodeJSONResponse(&list)); err != nil {
			return nil, err
		}

		return &list, nil
	})
}

type clusterCreateProgress struct {
	status atomic.String
}

func (crp *clusterCreateProgress) set(status string)      { crp.status.Store(status) }
func (crp *clusterCreateProgress) FormatProgress() string { return crp.status.Load() }

func reparse(obj interface{}, target interface{}) error {
	b, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	return json.Unmarshal(b, target)
}
