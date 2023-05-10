// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package api

import (
	"bufio"
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
	"google.golang.org/protobuf/types/known/emptypb"
	"namespacelabs.dev/foundation/framework/jsonreparser"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
)

type API struct {
	StartCreateKubernetesCluster fnapi.Call[CreateKubernetesClusterRequest]
	CreateContainers             fnapi.Call[CreateContainersRequest]
	StartContainers              fnapi.Call[StartContainersRequest]
	GetKubernetesCluster         fnapi.Call[GetKubernetesClusterRequest]
	EnsureKubernetesCluster      fnapi.Call[EnsureKubernetesClusterRequest]
	WaitKubernetesCluster        fnapi.Call[WaitKubernetesClusterRequest]
	ListKubernetesClusters       fnapi.Call[ListKubernetesClustersRequest]
	DestroyKubernetesCluster     fnapi.Call[DestroyKubernetesClusterRequest]
	ReleaseKubernetesCluster     fnapi.Call[ReleaseKubernetesClusterRequest]
	WakeKubernetesCluster        fnapi.Call[WakeKubernetesClusterRequest]
	RefreshKubernetesCluster     fnapi.Call[RefreshKubernetesClusterRequest]
	GetKubernetesClusterSummary  fnapi.Call[GetKubernetesClusterSummaryRequest]
	GetKubernetesConfig          fnapi.Call[GetKubernetesConfigRequest]
	GetImageRegistry             fnapi.Call[emptypb.Empty]
	TailClusterLogs              fnapi.Call[TailLogsRequest]
	GetClusterLogs               fnapi.Call[GetLogsRequest]
	GetProfile                   fnapi.Call[emptypb.Empty]
	RegisterDefaultIngress       fnapi.Call[RegisterDefaultIngressRequest]
}

var Endpoint API

var (
	rpcEndpointOverride string
	regionName          string
)

func SetupFlags(prefix string, flags *pflag.FlagSet, hide bool) {
	endpointFlag := fmt.Sprintf("%sendpoint", prefix)
	regionFlag := fmt.Sprintf("%sregion", prefix)

	flags.StringVar(&rpcEndpointOverride, endpointFlag, "", "Where to dial to when reaching nscloud.")
	flags.StringVar(&regionName, regionFlag, "fra1", "Which region to use.")

	if hide {
		_ = flags.MarkHidden(endpointFlag)
		_ = flags.MarkHidden(regionFlag)
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
	return API{
		StartCreateKubernetesCluster: fnapi.Call[CreateKubernetesClusterRequest]{
			Endpoint:   endpoint,
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/StartCreateKubernetesCluster",
		},

		CreateContainers: fnapi.Call[CreateContainersRequest]{
			Endpoint:   endpoint,
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/CreateContainers",
		},

		StartContainers: fnapi.Call[StartContainersRequest]{
			Endpoint:   endpoint,
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/StartContainers",
		},

		GetKubernetesCluster: fnapi.Call[GetKubernetesClusterRequest]{
			Endpoint:   endpoint,
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/GetKubernetesCluster",
		},

		EnsureKubernetesCluster: fnapi.Call[EnsureKubernetesClusterRequest]{
			Endpoint:   endpoint,
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/EnsureKubernetesCluster",
		},

		WaitKubernetesCluster: fnapi.Call[WaitKubernetesClusterRequest]{
			Endpoint:   endpoint,
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/WaitKubernetesCluster",
		},

		ListKubernetesClusters: fnapi.Call[ListKubernetesClustersRequest]{
			Endpoint:   endpoint,
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/ListKubernetesClusters",
		},

		DestroyKubernetesCluster: fnapi.Call[DestroyKubernetesClusterRequest]{
			Endpoint:   endpoint,
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/DestroyKubernetesCluster",
		},

		ReleaseKubernetesCluster: fnapi.Call[ReleaseKubernetesClusterRequest]{
			Endpoint:   endpoint,
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/ReleaseKubernetesCluster",
		},

		WakeKubernetesCluster: fnapi.Call[WakeKubernetesClusterRequest]{
			Endpoint:   endpoint,
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/WakeKubernetesCluster",
		},

		RefreshKubernetesCluster: fnapi.Call[RefreshKubernetesClusterRequest]{
			Endpoint:   endpoint,
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/RefreshKubernetesCluster",
		},

		GetKubernetesClusterSummary: fnapi.Call[GetKubernetesClusterSummaryRequest]{
			Endpoint:   endpoint,
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/GetKubernetesClusterSummary",
		},

		GetKubernetesConfig: fnapi.Call[GetKubernetesConfigRequest]{
			Endpoint:   endpoint,
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/GetKubernetesConfig",
		},

		GetImageRegistry: fnapi.Call[emptypb.Empty]{
			Endpoint:   endpoint,
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/GetImageRegistry",
		},

		TailClusterLogs: fnapi.Call[TailLogsRequest]{
			// XXX: hardcoded for now, we need to add an alias to api.<region>.nscluster.cloud
			Endpoint:   fmt.Sprintf("https://logging.nscloud-%s.namespacelabs.nscloud.dev", regionName),
			FetchToken: fnapi.FetchToken,
			Method:     "logs/tail",
		},

		GetClusterLogs: fnapi.Call[GetLogsRequest]{
			Endpoint:   endpoint,
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.logging.LoggingService/GetLogs",
		},

		GetProfile: fnapi.Call[emptypb.Empty]{
			Endpoint:   endpoint,
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/GetProfile",
		},

		RegisterDefaultIngress: fnapi.Call[RegisterDefaultIngressRequest]{
			Endpoint:   endpoint,
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/RegisterDefaultIngress",
		},
	}
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
	KeepAtExit bool

	Purpose           string
	Features          []string
	AuthorizedSshKeys []string
	UniqueTag         string
	InternalExtra     string

	WaitClusterOpts
}

type WaitClusterOpts struct {
	WaitKind string // One of kubernetes, buildcluster, or something else.

	WaitForService string
}

type EnsureBuildClusterOpts struct {
	Features []string
}

func CreateCluster(ctx context.Context, api API, opts CreateClusterOpts) (*StartCreateKubernetesClusterResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.cluster-create"), func(ctx context.Context) (*StartCreateKubernetesClusterResponse, error) {
		req := CreateKubernetesClusterRequest{
			Ephemeral:         opts.Ephemeral,
			DocumentedPurpose: opts.Purpose,
			MachineType:       opts.MachineType,
			Feature:           opts.Features,
			AuthorizedSshKeys: opts.AuthorizedSshKeys,
			UniqueTag:         opts.UniqueTag,
			InternalExtra:     opts.InternalExtra,
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

		if opts.Ephemeral && !opts.KeepAtExit {
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

	return WaitCluster(ctx, api, cluster.ClusterId, opts.WaitClusterOpts)
}

func EnsureBuildCluster(ctx context.Context, api API, opts EnsureBuildClusterOpts) (*CreateClusterResult, error) {
	featuresList := []string{"BUILD_CLUSTER"}
	featuresList = append(featuresList, opts.Features...)
	return CreateAndWaitCluster(ctx, api, CreateClusterOpts{
		Purpose:  "Build machine",
		Features: featuresList,
		WaitClusterOpts: WaitClusterOpts{
			WaitForService: "buildkit",
			WaitKind:       "buildcluster",
		},
	})
}

func WaitCluster(ctx context.Context, api API, clusterId string, opts WaitClusterOpts) (*CreateClusterResult, error) {
	ctx, done := context.WithTimeout(ctx, 15*time.Minute) // Wait for cluster creation up to 15 minutes.
	defer done()

	var cr *CreateKubernetesClusterResponse
	if err := tasks.Action("nscloud.cluster-wait").Arg("cluster_id", clusterId).Run(ctx, func(ctx context.Context) error {
		var progress clusterCreateProgress
		progress.status.Store(stageHumanLabel("CREATE_ACCEPTED_WAITING_FOR_ALLOCATION", opts.WaitKind))
		tasks.Attachments(ctx).SetProgress(&progress)

		lastStatus := "<none>"
		tries := 0
		for {
			// We continue to wait for the cluster to become ready until we observe a READY.
			if err := api.WaitKubernetesCluster.Do(ctx, WaitKubernetesClusterRequest{ClusterId: clusterId}, func(body io.Reader) error {
				// If we get a payload, reset the number of tries.
				tries = 0

				decoder := jstream.NewDecoder(body, 1)

				// jstream gives us the streamed array segmentation, however it
				// returns map[string]interface{} rather than typed objects. We
				// re-trigger parsing into the response type so the remainder of
				// our codebase operates on types.

				for mv := range decoder.Stream() {
					var resp CreateKubernetesClusterResponse
					if err := jsonreparser.Reparse(mv.Value, &resp); err != nil {
						return fnerrors.InvocationError("nscloud", "failed to parse response: %w", err)
					}

					progress.set(stageHumanLabel(resp.Status, opts.WaitKind))
					lastStatus = resp.Status

					if resp.ClusterId != "" {
						clusterId = resp.ClusterId
					}

					ready := resp.Status == "READY"
					if opts.WaitForService != "" {
						svc := ClusterService(resp.Cluster, opts.WaitForService)
						if svc != nil && svc.Status == "READY" {
							ready = true
						}
					}

					if ready {
						cr = &resp
						return nil
					}
				}

				return fnerrors.InvocationError("nscloud", "stream closed before cluster became ready")
			}); err != nil {
				tries++
				if tries >= 3 {
					return fnerrors.InvocationError("nscloud", "cluster never became ready (last status was %q, cluster id: %s): %w", lastStatus, clusterId, err)
				}
			}

			if cr != nil {
				break
			}

			if err := ctx.Err(); err != nil {
				return err // Check if we've been cancelled.
			}

			time.Sleep(time.Second)
		}

		if cr.Cluster.EndpointAddress != "" {
			tasks.Attachments(ctx).AddResult("cluster_address", cr.Cluster.EndpointAddress)
		}

		tasks.Attachments(ctx).AddResult("deadline", cr.Cluster.Deadline)

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

func stageHumanLabel(stage, kind string) string {
	switch stage {
	case "CREATE_ACCEPTED_WAITING_FOR_ALLOCATION":
		return "Waiting for resources"

	case "COMMITTED":
		return "Assigning machine"

	case "NAMESPACE_CREATED":
		return "Starting isolated environment"

	case "NAMESPACE_READY":
		return "Waiting for core services"

	case "CORE_SERVICES_READY":
		switch kind {
		case "kubernetes":
			return "Waiting for Kubernetes"

		case "buildcluster":
			return "Waiting for build cluster"

		default:
			return "Waiting for containers"
		}

	case "KUBERNETES_READY", "KUBERNETES_LOAD_BALANCER_READY":
		return "Waiting to settle"

	case "BUILD_CLUSTER_READY":
		return "Waiting to settle"
	}

	return stage
}

func ClusterService(cluster *KubernetesCluster, name string) *Cluster_ServiceState {
	if cluster == nil {
		return nil
	}

	for _, srv := range cluster.ServiceState {
		if srv.Name == name {
			return srv
		}
	}

	return nil
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

func EnsureCluster(ctx context.Context, api API, clusterId string) (*GetKubernetesClusterResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.ensure").Arg("id", clusterId), func(ctx context.Context) (*GetKubernetesClusterResponse, error) {
		var response GetKubernetesClusterResponse
		if err := api.EnsureKubernetesCluster.Do(ctx, EnsureKubernetesClusterRequest{ClusterId: clusterId}, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}
		return &response, nil
	})
}

func GetClusterSummary(ctx context.Context, api API, clusterId string, resources []string) (*GetKubernetesClusterSummaryResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.get-summary").Arg("id", clusterId), func(ctx context.Context) (*GetKubernetesClusterSummaryResponse, error) {
		var response GetKubernetesClusterSummaryResponse
		if err := api.GetKubernetesClusterSummary.Do(ctx, GetKubernetesClusterSummaryRequest{ClusterId: clusterId, Resource: resources}, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}
		return &response, nil
	})
}

func GetKubernetesConfig(ctx context.Context, api API, clusterId string) (*GetKubernetesConfigResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.get").Arg("id", clusterId), func(ctx context.Context) (*GetKubernetesConfigResponse, error) {
		var response GetKubernetesConfigResponse
		if err := api.GetKubernetesConfig.Do(ctx, GetKubernetesConfigRequest{ClusterId: clusterId}, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}
		return &response, nil
	})
}

func GetImageRegistry(ctx context.Context, api API) (*GetImageRegistryResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.get-image-registry"), func(ctx context.Context) (*GetImageRegistryResponse, error) {
		var response GetImageRegistryResponse
		if err := api.GetImageRegistry.Do(ctx, emptypb.Empty{}, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}
		return &response, nil
	})
}

type ListOpts struct {
	PreviousRuns bool
	NotOlderThan *time.Time
	Labels       map[string]string
}

func ListClusters(ctx context.Context, api API, opts ListOpts) (*ListKubernetesClustersResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.cluster-list"), func(ctx context.Context) (*ListKubernetesClustersResponse, error) {
		req := ListKubernetesClustersRequest{
			IncludePreviousRuns: opts.PreviousRuns,
			NotOlderThan:        opts.NotOlderThan,
		}

		for key, value := range opts.Labels {
			req.LabelFilter = append(req.LabelFilter, &LabelFilterEntry{
				Name:  key,
				Value: value,
				Op:    "EQUAL",
			})
		}

		var list ListKubernetesClustersResponse
		if err := api.ListKubernetesClusters.Do(ctx, req, fnapi.DecodeJSONResponse(&list)); err != nil {
			return nil, err
		}

		return &list, nil
	})
}

type LogsOpts struct {
	ClusterID string
	StartTs   *time.Time
	EndTs     *time.Time
	Include   []*LogsSelector
	Exclude   []*LogsSelector
}

func TailClusterLogs(ctx context.Context, api API, opts *LogsOpts, handle func(LogBlock) error) error {
	return api.TailClusterLogs.Do(ctx, TailLogsRequest{
		ClusterID: opts.ClusterID,
		Include:   opts.Include,
		Exclude:   opts.Exclude,
	}, func(r io.Reader) error {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			var logBlock LogBlock
			data := scanner.Bytes()
			if err := json.Unmarshal(data, &logBlock); err != nil {
				fmt.Fprintf(console.Debug(ctx), "Failed to process a log entry %s: %v\n", string(data), err)
				continue
			}

			if handle != nil {
				if err := handle(logBlock); err != nil {
					return err
				}
			}
		}

		if err := scanner.Err(); err != nil {
			return err
		}
		return nil
	})
}

func GetClusterLogs(ctx context.Context, api API, opts *LogsOpts) (*GetLogsResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.get-cluster-logs"), func(ctx context.Context) (*GetLogsResponse, error) {
		req := GetLogsRequest{
			ClusterID: opts.ClusterID,
			StartTs:   opts.StartTs,
			EndTs:     opts.EndTs,
			Include:   opts.Include,
			Exclude:   opts.Exclude,
		}

		var response GetLogsResponse
		if err := api.GetClusterLogs.Do(ctx, req, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}

		return &response, nil
	})
}

func GetProfile(ctx context.Context, api API) (*GetProfileResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.get-profile"), func(ctx context.Context) (*GetProfileResponse, error) {
		var response GetProfileResponse
		if err := api.GetProfile.Do(ctx, emptypb.Empty{}, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}
		return &response, nil
	})
}

func RegisterDefaultIngress(ctx context.Context, api API, req RegisterDefaultIngressRequest) (*RegisterDefaultIngressResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.register-ingress"), func(ctx context.Context) (*RegisterDefaultIngressResponse, error) {
		var response RegisterDefaultIngressResponse
		if err := api.RegisterDefaultIngress.Do(ctx, req, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}
		return &response, nil
	})
}

type clusterCreateProgress struct {
	status atomic.String
}

func (crp *clusterCreateProgress) set(status string)      { crp.status.Store(status) }
func (crp *clusterCreateProgress) FormatProgress() string { return crp.status.Load() }

func RefreshCluster(ctx context.Context, api API, clusterId string) (*RefreshKubernetesClusterResponse, error) {
	var response RefreshKubernetesClusterResponse
	if err := api.RefreshKubernetesCluster.Do(ctx, RefreshKubernetesClusterRequest{
		ClusterId: clusterId,
	}, fnapi.DecodeJSONResponse(&response)); err != nil {
		return nil, err
	}
	return &response, nil
}

func StartRefreshing(ctx context.Context, api API, clusterId string, handle func(error) error) error {
	for {
		if _, err := RefreshCluster(ctx, api, clusterId); err != nil {
			if err := handle(err); err != nil {
				return err
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Minute):
			}
		} else {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(10 * time.Minute):
			}
		}
	}
}
