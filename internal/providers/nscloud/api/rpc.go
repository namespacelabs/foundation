// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package api

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/bcicen/jstream"
	"github.com/dustin/go-humanize"
	"go.uber.org/atomic"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/environment"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
)

const machineEndpoint = "https://grpc-gateway-84umfjt8rm05f5dimftg.prod-metal.namespacelabs.nscloud.dev"

var (
	startCreateKubernetesCluster = fnapi.Call[CreateKubernetesClusterRequest]{
		Endpoint: machineEndpoint,
		Method:   "nsl.vm.api.VMService/StartCreateKubernetesCluster",
		PreAuthenticateRequest: func(user *fnapi.UserAuth, rt *CreateKubernetesClusterRequest) error {
			return fillIn(user, &rt.OpaqueUserAuth)
		},
	}

	getKubernetesCluster = fnapi.Call[GetKubernetesClusterRequest]{
		Endpoint: machineEndpoint,
		Method:   "nsl.vm.api.VMService/GetKubernetesCluster",
		PreAuthenticateRequest: func(user *fnapi.UserAuth, rt *GetKubernetesClusterRequest) error {
			return fillIn(user, &rt.OpaqueUserAuth)
		},
	}

	waitKubernetesCluster = fnapi.Call[WaitKubernetesClusterRequest]{
		Endpoint: machineEndpoint,
		Method:   "nsl.vm.api.VMService/WaitKubernetesCluster",
		PreAuthenticateRequest: func(user *fnapi.UserAuth, rt *WaitKubernetesClusterRequest) error {
			return fillIn(user, &rt.OpaqueUserAuth)
		},
	}

	listKubernetesClusters = fnapi.Call[ListKubernetesClustersRequest]{
		Endpoint: machineEndpoint,
		Method:   "nsl.vm.api.VMService/ListKubernetesClusters",
		PreAuthenticateRequest: func(user *fnapi.UserAuth, rt *ListKubernetesClustersRequest) error {
			return fillIn(user, &rt.OpaqueUserAuth)
		},
	}

	destroyKubernetesCluster = fnapi.Call[DestroyKubernetesClusterRequest]{
		Endpoint: machineEndpoint,
		Method:   "nsl.vm.api.VMService/DestroyKubernetesCluster",
		PreAuthenticateRequest: func(user *fnapi.UserAuth, rt *DestroyKubernetesClusterRequest) error {
			return fillIn(user, &rt.OpaqueUserAuth)
		},
	}
)

type CreateClusterResult struct {
	ClusterId    string
	Cluster      *KubernetesCluster
	Registry     *ImageRegistry
	BuildCluster *BuildCluster
	Deadline     *time.Time
}

func CreateCluster(ctx context.Context, machineType string, ephemeral bool, purpose string, features []string) (*KubernetesCluster, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.cluster-create"), func(ctx context.Context) (*KubernetesCluster, error) {
		req := CreateKubernetesClusterRequest{
			Ephemeral:         ephemeral,
			DocumentedPurpose: purpose,
			MachineType:       machineType,
			Feature:           features,
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

				req.AuthorizedSshKeys = actualKeys
			}
		}

		var response StartCreateKubernetesClusterResponse
		if err := startCreateKubernetesCluster.Do(ctx, req, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}

		tasks.Attachments(ctx).AddResult("cluster_id", response.ClusterId)

		if response.ClusterFragment != nil {
			if shape := response.ClusterFragment.Shape; shape != nil {
				tasks.Attachments(ctx).
					AddResult("cluster_cpu", shape.VirtualCpu).
					AddResult("cluster_ram", humanize.IBytes(uint64(shape.MemoryMegabytes)*humanize.MiByte))
			}
		}

		if ephemeral {
			compute.On(ctx).Cleanup(tasks.Action("nscloud.cluster-cleanup"), func(ctx context.Context) error {
				if err := DestroyCluster(ctx, response.ClusterId); err != nil {
					// The cluster being gone is an acceptable state (it could have
					// been deleted by DeleteRecursively for example).
					if status.Code(err) == codes.NotFound {
						return nil
					}
				}

				return nil
			})
		}

		return response.ClusterFragment, nil
	})
}

func CreateAndWaitCluster(ctx context.Context, machineType string, ephemeral bool, purpose string, features []string) (*CreateClusterResult, error) {
	cluster, err := CreateCluster(ctx, machineType, ephemeral, purpose, features)
	if err != nil {
		return nil, err
	}

	return WaitCluster(ctx, cluster.ClusterId)
}

func WaitCluster(ctx context.Context, clusterId string) (*CreateClusterResult, error) {
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
			if err := waitKubernetesCluster.Do(ctx, WaitKubernetesClusterRequest{ClusterId: clusterId}, func(body io.Reader) error {
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

				return nil
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

func DestroyCluster(ctx context.Context, clusterId string) error {
	return destroyKubernetesCluster.Do(ctx, DestroyKubernetesClusterRequest{
		ClusterId: clusterId,
	}, nil)
}

func GetCluster(ctx context.Context, clusterId string) (*GetKubernetesClusterResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.get").Arg("id", clusterId), func(ctx context.Context) (*GetKubernetesClusterResponse, error) {
		var response GetKubernetesClusterResponse
		if err := getKubernetesCluster.Do(ctx, GetKubernetesClusterRequest{ClusterId: clusterId}, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}
		return &response, nil
	})
}

func ListClusters(ctx context.Context) (*KubernetesClusterList, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.cluster-list"), func(ctx context.Context) (*KubernetesClusterList, error) {
		var list KubernetesClusterList
		if err := listKubernetesClusters.Do(ctx, ListKubernetesClustersRequest{}, fnapi.DecodeJSONResponse(&list)); err != nil {
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

func fillIn(userAuth *fnapi.UserAuth, target *[]byte) error {
	if userAuth.InternalOpaque != nil {
		*target = userAuth.InternalOpaque
	}
	return nil
}
