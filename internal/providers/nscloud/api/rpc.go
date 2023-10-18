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
	"os"
	"regexp"
	"time"

	"github.com/bcicen/jstream"
	"github.com/dustin/go-humanize"
	"github.com/spf13/pflag"
	"go.uber.org/atomic"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	SuspendKubernetesCluster     fnapi.Call[SuspendKubernetesClusterRequest]
	ReleaseKubernetesCluster     fnapi.Call[ReleaseKubernetesClusterRequest]
	WakeKubernetesCluster        fnapi.Call[WakeKubernetesClusterRequest]
	RefreshKubernetesCluster     fnapi.Call[RefreshKubernetesClusterRequest]
	GetKubernetesClusterSummary  fnapi.Call[GetKubernetesClusterSummaryRequest]
	GetKubernetesConfig          fnapi.Call[GetKubernetesConfigRequest]
	GetImageRegistry             fnapi.Call[emptypb.Empty]
	TailClusterLogs              fnapi.Call[TailLogsRequest]
	GetClusterLogs               fnapi.Call[GetLogsRequest]
	GetProfile                   fnapi.Call[emptypb.Empty]
	RegisterIngress              fnapi.Call[RegisterIngressRequest]
	ListIngresses                fnapi.Call[ListIngressesRequest]
	ListVolumes                  fnapi.Call[emptypb.Empty]
	DestroyVolumeByTag           fnapi.Call[DestroyVolumeByTagRequest]
}

var (
	Methods = MakeAPI()

	rpcEndpointOverride string
	RegionName          string
)

func SetupFlags(prefix string, flags *pflag.FlagSet, hide bool) {
	endpointFlag := fmt.Sprintf("%sendpoint", prefix)
	regionFlag := fmt.Sprintf("%sregion", prefix)

	flags.StringVar(&rpcEndpointOverride, endpointFlag, "", "Where to dial to when reaching nscloud.")
	flags.StringVar(&RegionName, regionFlag, "eu", "Which region to use.")

	if hide {
		_ = flags.MarkHidden(endpointFlag)
		_ = flags.MarkHidden(regionFlag)
	}
}

func ResolveEndpoint(ctx context.Context, tok fnapi.Token) (string, error) {
	if rpcEndpointOverride != "" {
		return rpcEndpointOverride, nil
	}

	if rpcEndpoint := os.Getenv("NSC_ENDPOINT"); rpcEndpoint != "" {
		return rpcEndpoint, nil
	}

	if tok == nil {
		return "", fnerrors.New("a token is required")
	}

	claims, err := fnapi.Claims(tok)
	if err != nil {
		return "", err
	}

	if claims.PrimaryRegion != "" {
		return "https://api." + claims.PrimaryRegion, nil
	}

	rpcEndpoint := fmt.Sprintf("https://api.%s.nscluster.cloud", RegionName)

	return rpcEndpoint, nil
}

func MakeAPI() API {
	return API{
		StartCreateKubernetesCluster: fnapi.Call[CreateKubernetesClusterRequest]{
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/StartCreateKubernetesCluster",
		},

		CreateContainers: fnapi.Call[CreateContainersRequest]{
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/CreateContainers",
		},

		StartContainers: fnapi.Call[StartContainersRequest]{
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/StartContainers",
		},

		GetKubernetesCluster: fnapi.Call[GetKubernetesClusterRequest]{
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/GetKubernetesCluster",
		},

		EnsureKubernetesCluster: fnapi.Call[EnsureKubernetesClusterRequest]{
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/EnsureKubernetesCluster",
		},

		WaitKubernetesCluster: fnapi.Call[WaitKubernetesClusterRequest]{
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/WaitKubernetesCluster",
		},

		ListKubernetesClusters: fnapi.Call[ListKubernetesClustersRequest]{
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/ListKubernetesClusters",
		},

		DestroyKubernetesCluster: fnapi.Call[DestroyKubernetesClusterRequest]{
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/DestroyKubernetesCluster",
		},

		SuspendKubernetesCluster: fnapi.Call[SuspendKubernetesClusterRequest]{
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/SuspendKubernetesCluster",
		},

		ReleaseKubernetesCluster: fnapi.Call[ReleaseKubernetesClusterRequest]{
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/ReleaseKubernetesCluster",
		},

		WakeKubernetesCluster: fnapi.Call[WakeKubernetesClusterRequest]{
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/WakeKubernetesCluster",
		},

		RefreshKubernetesCluster: fnapi.Call[RefreshKubernetesClusterRequest]{
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/RefreshKubernetesCluster",
		},

		GetKubernetesClusterSummary: fnapi.Call[GetKubernetesClusterSummaryRequest]{
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/GetKubernetesClusterSummary",
		},

		GetKubernetesConfig: fnapi.Call[GetKubernetesConfigRequest]{
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/GetKubernetesConfig",
		},

		GetImageRegistry: fnapi.Call[emptypb.Empty]{
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/GetImageRegistry",
		},

		TailClusterLogs: fnapi.Call[TailLogsRequest]{
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.logging.LoggingService/TailLogs",
		},

		GetClusterLogs: fnapi.Call[GetLogsRequest]{
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.logging.LoggingService/GetLogs",
		},

		GetProfile: fnapi.Call[emptypb.Empty]{
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/GetProfile",
		},

		RegisterIngress: fnapi.Call[RegisterIngressRequest]{
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/RegisterIngress",
		},

		ListIngresses: fnapi.Call[ListIngressesRequest]{
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/ListIngresses",
		},

		ListVolumes: fnapi.Call[emptypb.Empty]{
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/ListVolumes",
		},

		DestroyVolumeByTag: fnapi.Call[DestroyVolumeByTagRequest]{
			FetchToken: fnapi.FetchToken,
			Method:     "nsl.vm.api.VMService/DestroyVolumeByTag",
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

	// Whether to keep the cluster alive, regardless of it being ephemeral.
	// This is typically needed if you want to execute multiple ns commands on an ephemeral cluster.
	KeepAtExit bool

	Purpose           string
	Features          []string
	AuthorizedSshKeys []string
	UniqueTag         string
	InternalExtra     string
	Labels            map[string]string
	Deadline          *timestamppb.Timestamp
	Experimental      any
	SecretIDs         []string

	WaitClusterOpts
}

type WaitClusterOpts struct {
	CreateLabel string // Used as human-facing label, e.g. "Creating Environment: ..."

	WaitKind string // One of kubernetes, buildcluster, or something else.

	WaitForService string
}

func (w WaitClusterOpts) label() string {
	if w.CreateLabel == "" {
		return "Creating Environment"
	}

	return w.CreateLabel
}

func CreateCluster(ctx context.Context, api API, opts CreateClusterOpts) (*StartCreateKubernetesClusterResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.cluster-create").HumanReadablef(opts.label()), func(ctx context.Context) (*StartCreateKubernetesClusterResponse, error) {
		req := CreateKubernetesClusterRequest{
			DocumentedPurpose: opts.Purpose,
			MachineType:       opts.MachineType,
			Feature:           opts.Features,
			AuthorizedSshKeys: opts.AuthorizedSshKeys,
			UniqueTag:         opts.UniqueTag,
			InternalExtra:     opts.InternalExtra,
			Deadline:          opts.Deadline,
			Experimental:      opts.Experimental,
			Interactive:       true,
		}

		labelKeys := maps.Keys(opts.Labels)
		slices.Sort(labelKeys)
		for _, key := range labelKeys {
			req.Label = append(req.Label, &LabelEntry{
				Name:  key,
				Value: opts.Labels[key],
			})
		}

		for _, sid := range opts.SecretIDs {
			req.AvailableSecrets = append(req.AvailableSecrets, &SecretRef{SecretID: sid})
		}

		var response StartCreateKubernetesClusterResponse
		if err := api.StartCreateKubernetesCluster.Do(ctx, req, ResolveEndpoint, fnapi.DecodeJSONResponse(&response)); err != nil {
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

		fmt.Fprintf(console.Debug(ctx), "[nsc] created instance: %s\n", response.ClusterId)

		if !opts.KeepAtExit {
			fmt.Fprintf(console.Debug(ctx), "[nsc] instance will be removed on exit: %s\n", response.ClusterId)

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

	return WaitClusterReady(ctx, api, cluster.ClusterId, opts.WaitClusterOpts)
}

func CreateBuildCluster(ctx context.Context, api API, platform BuildPlatform) (*CreateClusterResult, error) {
	featuresList := []string{"BUILD_CLUSTER"}
	featuresList = append(featuresList, buildClusterFeatures(platform)...)
	return CreateAndWaitCluster(ctx, api, CreateClusterOpts{
		Purpose:    "Build machine",
		Features:   featuresList,
		KeepAtExit: true,
		WaitClusterOpts: WaitClusterOpts{
			CreateLabel:    fmt.Sprintf("Creating %s Build Cluster", platform),
			WaitForService: "buildkit",
			WaitKind:       "buildcluster",
		},
	})
}

func buildClusterFeatures(platform BuildPlatform) []string {
	if platform == "arm64" {
		return []string{"EXP_ARM64_CLUSTER"}
	}

	return nil
}

func WaitClusterReady(ctx context.Context, api API, clusterId string, opts WaitClusterOpts) (*CreateClusterResult, error) {
	ctx, done := context.WithTimeout(ctx, 1*time.Minute) // Wait for cluster creation up to 1 minute.
	defer done()

	var cr *CreateKubernetesClusterResponse
	if err := tasks.Action("nscloud.cluster-wait").HumanReadablef(opts.label()).Arg("cluster_id", clusterId).Run(ctx, func(ctx context.Context) error {
		var progress clusterCreateProgress
		progress.status.Store(stageHumanLabel("CREATE_ACCEPTED_WAITING_FOR_ALLOCATION", opts.WaitKind))
		tasks.Attachments(ctx).SetProgress(&progress)

		lastStatus := "<none>"
		tries := 0
		for {
			// We continue to wait for the cluster to become ready until we observe a READY.
			if err := api.WaitKubernetesCluster.Do(ctx, WaitKubernetesClusterRequest{ClusterId: clusterId}, ResolveEndpoint, func(body io.Reader) error {
				decoder := jstream.NewDecoder(body, 1)

				// jstream gives us the streamed array segmentation, however it
				// returns map[string]interface{} rather than typed objects. We
				// re-trigger parsing into the response type so the remainder of
				// our codebase operates on types.

				for mv := range decoder.Stream() {
					// If we get a payload, reset the number of tries.
					tries = 0

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

					if resp.Cluster.DestroyedAt != "" {
						// Cluster was destroyed
						cr = &resp
						return fnerrors.InvocationError("nscloud", "cluster is destroyed (cluster id: %s)", clusterId)
					}
				}

				return fnerrors.InvocationError("nscloud", "stream closed before cluster became ready")
			}); err != nil {
				tries++
				if tries >= 3 {
					return fnerrors.InvocationError("nscloud", "cluster never became ready (last status was %q, cluster id: %s): %w", lastStatus, clusterId, err)
				} else {
					fmt.Fprintf(console.Debug(ctx), "Failed to wait for cluster: %v\n", err)
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
	}, ResolveEndpoint, nil)
}

func GetCluster(ctx context.Context, api API, clusterId string) (*GetKubernetesClusterResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.get").Arg("id", clusterId), func(ctx context.Context) (*GetKubernetesClusterResponse, error) {
		var response GetKubernetesClusterResponse
		if err := api.GetKubernetesCluster.Do(ctx, GetKubernetesClusterRequest{ClusterId: clusterId}, ResolveEndpoint, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}
		return &response, nil
	})
}

func EnsureCluster(ctx context.Context, api API, clusterId string) (*GetKubernetesClusterResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.ensure").HumanReadablef("Waiting for environment").Arg("id", clusterId), func(ctx context.Context) (*GetKubernetesClusterResponse, error) {
		var response GetKubernetesClusterResponse
		if err := api.EnsureKubernetesCluster.Do(ctx, EnsureKubernetesClusterRequest{ClusterId: clusterId}, ResolveEndpoint, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}
		return &response, nil
	})
}

func GetClusterSummary(ctx context.Context, api API, clusterId string, resources []string) (*GetKubernetesClusterSummaryResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.get-summary").LogLevel(1).Arg("id", clusterId), func(ctx context.Context) (*GetKubernetesClusterSummaryResponse, error) {
		var response GetKubernetesClusterSummaryResponse
		if err := api.GetKubernetesClusterSummary.Do(ctx, GetKubernetesClusterSummaryRequest{ClusterId: clusterId, Resource: resources}, ResolveEndpoint, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}
		return &response, nil
	})
}

func GetKubernetesConfig(ctx context.Context, api API, clusterId string) (*GetKubernetesConfigResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.get").Arg("id", clusterId), func(ctx context.Context) (*GetKubernetesConfigResponse, error) {
		var response GetKubernetesConfigResponse
		if err := api.GetKubernetesConfig.Do(ctx, GetKubernetesConfigRequest{ClusterId: clusterId}, ResolveEndpoint, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}
		return &response, nil
	})
}

func GetImageRegistry(ctx context.Context, api API) (*GetImageRegistryResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.get-image-registry"), func(ctx context.Context) (*GetImageRegistryResponse, error) {
		var response GetImageRegistryResponse
		if err := api.GetImageRegistry.Do(ctx, emptypb.Empty{}, ResolveEndpoint, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}
		return &response, nil
	})
}

type ListOpts struct {
	PreviousRuns bool
	NotOlderThan *time.Time
	Labels       map[string]string
	All          bool
}

func ListClusters(ctx context.Context, api API, opts ListOpts) (*ListKubernetesClustersResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.cluster-list"), func(ctx context.Context) (*ListKubernetesClustersResponse, error) {
		req := ListKubernetesClustersRequest{
			IncludePreviousRuns: opts.PreviousRuns,
			NotOlderThan:        opts.NotOlderThan,
			KindFilter:          "MANUAL_ONLY",
		}

		if opts.All {
			req.KindFilter = "RETURN_ALL"
		}

		for key, value := range opts.Labels {
			req.LabelFilter = append(req.LabelFilter, &LabelFilterEntry{
				Name:  key,
				Value: value,
				Op:    "EQUAL",
			})
		}

		var list ListKubernetesClustersResponse
		if err := api.ListKubernetesClusters.Do(ctx, req, ResolveEndpoint, fnapi.DecodeJSONResponse(&list)); err != nil {
			return nil, err
		}

		return &list, nil
	})
}

type LogsOpts struct {
	ClusterID     string
	StartTs       *time.Time
	EndTs         *time.Time
	Include       []*LogsSelector
	Exclude       []*LogsSelector
	IngressDomain string
}

var (
	streamResetError = regexp.MustCompile("^stream error:.*; received from peer$")
)

func TailClusterLogs(ctx context.Context, api API, opts *LogsOpts, handle func(LogBlock) error) error {
	return api.TailClusterLogs.Do(ctx, TailLogsRequest{
		ClusterID:      opts.ClusterID,
		UseBlockLabels: true,
		Include:        opts.Include,
		Exclude:        opts.Exclude,
	}, ResolveEndpoint, func(r io.Reader) error {
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
			// XXX Replace with pagination
			if streamResetError.MatchString(err.Error()) {
				return fnerrors.New("Logs stream reset. We saw: %w\n\nThis can happen if no new logs arrived for a long time.\nIf you are still expecting new logs, please retry.", err)
			}

			return fnerrors.New("cluster log stream closed with error: %w", err)
		}
		return nil
	})
}

func GetClusterLogs(ctx context.Context, api API, opts *LogsOpts) (*GetLogsResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.get-cluster-logs"), func(ctx context.Context) (*GetLogsResponse, error) {
		req := GetLogsRequest{
			ClusterID:      opts.ClusterID,
			UseBlockLabels: true,
			StartTs:        opts.StartTs,
			EndTs:          opts.EndTs,
			Include:        opts.Include,
			Exclude:        opts.Exclude,
		}

		resolveEndpoint := func(ctx context.Context, tok fnapi.Token) (string, error) {
			if opts.IngressDomain != "" {
				return fmt.Sprintf("https://api.%s", opts.IngressDomain), nil
			}

			return ResolveEndpoint(ctx, tok)
		}

		var response GetLogsResponse
		if err := api.GetClusterLogs.Do(ctx, req, resolveEndpoint, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}

		return &response, nil
	})
}

func GetProfile(ctx context.Context, api API) (*GetProfileResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.get-profile"), func(ctx context.Context) (*GetProfileResponse, error) {
		var response GetProfileResponse
		if err := api.GetProfile.Do(ctx, emptypb.Empty{}, ResolveEndpoint, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}
		return &response, nil
	})
}

func RegisterIngress(ctx context.Context, api API, req RegisterIngressRequest) (*RegisterIngressResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.register-ingress"), func(ctx context.Context) (*RegisterIngressResponse, error) {
		var response RegisterIngressResponse
		if err := api.RegisterIngress.Do(ctx, req, ResolveEndpoint, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}
		return &response, nil
	})
}

func ListIngresses(ctx context.Context, api API, clusterID string) (*ListIngressesResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.list-ingresses"), func(ctx context.Context) (*ListIngressesResponse, error) {
		var response ListIngressesResponse
		if err := api.ListIngresses.Do(ctx, ListIngressesRequest{ClusterId: clusterID}, ResolveEndpoint, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}
		return &response, nil
	})
}

func ListVolumes(ctx context.Context, api API) (*ListVolumesResponse, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.list-volumes"), func(ctx context.Context) (*ListVolumesResponse, error) {
		var response ListVolumesResponse
		if err := api.ListVolumes.Do(ctx, emptypb.Empty{}, ResolveEndpoint, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}
		return &response, nil
	})
}

func DestroyVolumeByTag(ctx context.Context, api API, tag string) error {
	return tasks.Return0(ctx, tasks.Action("nscloud.destroy-volumes"), func(ctx context.Context) error {
		if err := api.DestroyVolumeByTag.Do(ctx, DestroyVolumeByTagRequest{Tag: tag}, ResolveEndpoint, nil); err != nil {
			return err
		}
		return nil
	})
}

type clusterCreateProgress struct {
	status atomic.String
}

func (crp *clusterCreateProgress) set(status string)      { crp.status.Store(status) }
func (crp *clusterCreateProgress) FormatProgress() string { return crp.status.Load() }

func RefreshCluster(ctx context.Context, api API, req RefreshKubernetesClusterRequest) (*RefreshKubernetesClusterResponse, error) {
	var response RefreshKubernetesClusterResponse
	if err := api.RefreshKubernetesCluster.Do(ctx, req, ResolveEndpoint, fnapi.DecodeJSONResponse(&response)); err != nil {
		return nil, err
	}
	return &response, nil
}

func StartRefreshing(ctx context.Context, api API, clusterId string, handle func(error) error) error {
	for {
		if _, err := RefreshCluster(ctx, api, RefreshKubernetesClusterRequest{ClusterId: clusterId}); err != nil {
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
