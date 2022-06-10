// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"

	accesslogv3 "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	filev3 "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/file/v3"
	grpcjsontranscoder "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/grpc_json_transcoder/v3"
	routerfilter "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"google.golang.org/protobuf/proto"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
)

const (
	ListenerName     = "listener_0"
	LocalRouteName   = "local_route"
	LocalVirtualHost = "local_virtual_host"
	StatPrefix       = "grpc_json"
)

type httpListenerConfig struct {
	name    string
	address string
	port    uint32
}

type transcoderWithCluster struct {
	transcoder  *HttpGrpcTranscoder
	clusterName string
}

type TranscoderSnapshot struct {
	// Guards access to data below.
	mu sync.Mutex

	// Configuration of the http listener built from the registered transcoders.
	httpConfig httpListenerConfig

	// Envoy Node that we set the snapshot for.
	envoyNodeId string

	// Monotonically increasing counter of cache snapshot identifiers.
	snapshotId int
	cache      cache.SnapshotCache

	// Maps fully qualified proto service names to the corresponding HttpGrpcTranscoder.
	transcoders map[string]*HttpGrpcTranscoder
}

func NewTranscoderSnapshot(envoyNodeId string, logger Logger) *TranscoderSnapshot {
	cache := cache.NewSnapshotCache(false, cache.IDHash{}, logger)
	return &TranscoderSnapshot{
		envoyNodeId: envoyNodeId,
		snapshotId:  1,
		cache:       cache,
		transcoders: make(map[string]*HttpGrpcTranscoder),
	}
}

func (t *TranscoderSnapshot) RegisterHttpListener(listenerAddr string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	addr, portStr, err := net.SplitHostPort(listenerAddr)
	if err != nil {
		return err
	}

	port, err := strconv.ParseInt(portStr, 10, 32)
	if err != nil {
		return err
	}

	t.httpConfig = httpListenerConfig{ListenerName, addr, uint32(port)}
	return nil
}

// AddTranscoder adds a new transformer.
func (t *TranscoderSnapshot) AddTranscoder(transcoder *HttpGrpcTranscoder) {
	t.mu.Lock()
	t.transcoders[transcoder.Spec.FullyQualifiedProtoServiceName] = transcoder
	t.mu.Unlock()
}

// DeleteTranscoder deletes a transcoder.
func (t *TranscoderSnapshot) DeleteTranscoder(transcoder *HttpGrpcTranscoder) {
	t.mu.Lock()
	delete(t.transcoders, transcoder.Spec.FullyQualifiedProtoServiceName)
	t.mu.Unlock()
}

// GenerateSnapshot generates a new envoy snapshot of all registered transcoders.
func (t *TranscoderSnapshot) GenerateSnapshot(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	var transcoders []transcoderWithCluster

	var clusters []types.Resource
	for _, transcoder := range t.transcoders {
		clusterName := fmt.Sprintf("cluster-%s", strings.ReplaceAll(transcoder.Spec.FullyQualifiedProtoServiceName, ".", "-"))
		transcoders = append(transcoders, transcoderWithCluster{transcoder, clusterName})

		clusters = append(clusters, makeCluster(clusterName, transcoder.Spec))
	}

	httpListener, err := makeHTTPListener(t.httpConfig, transcoders)
	if err != nil {
		return fnerrors.InternalError("failed to create the http listener: %w", err)
	}

	snapshot, err := cache.NewSnapshot(strconv.Itoa(t.snapshotId),
		map[resource.Type][]types.Resource{
			resource.ClusterType:  clusters,
			resource.ListenerType: {httpListener},
		},
	)
	if err != nil {
		return err
	}

	if err := snapshot.Consistent(); err != nil {
		return fnerrors.InternalError("failed to generate a consistent snapshot: %w", err)
	}

	if err := t.cache.SetSnapshot(ctx, t.envoyNodeId, snapshot); err != nil {
		return fnerrors.InternalError("failed to set the snapshot: %w", err)
	}

	// Increment the snapshot identifier after verifying everything is consistent.
	t.snapshotId++

	return nil
}

func makeCluster(clusterName string, transcoderSpec HttpGrpcTranscoderSpec) *cluster.Cluster {
	return &cluster.Cluster{
		Name:                 clusterName,
		ConnectTimeout:       durationpb.New(60 * time.Second),
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_LOGICAL_DNS},
		LbPolicy:             cluster.Cluster_ROUND_ROBIN,
		LoadAssignment:       makeEndpoint(clusterName, transcoderSpec),
		DnsLookupFamily:      cluster.Cluster_V4_ONLY,
	}
}

func makeEndpoint(clusterName string, transcoderSpec HttpGrpcTranscoderSpec) *endpoint.ClusterLoadAssignment {
	return &endpoint.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints: []*endpoint.LocalityLbEndpoints{{
			LbEndpoints: []*endpoint.LbEndpoint{{
				HostIdentifier: &endpoint.LbEndpoint_Endpoint{
					Endpoint: &endpoint.Endpoint{
						Address: &core.Address{
							Address: &core.Address_SocketAddress{
								SocketAddress: &core.SocketAddress{
									Protocol: core.SocketAddress_TCP,
									Address:  transcoderSpec.ServiceAddress,
									PortSpecifier: &core.SocketAddress_PortValue{
										PortValue: transcoderSpec.ServicePort,
									},
								},
							},
						},
					},
				},
			}},
		}},
	}
}

func decodeBase64FiledescriptorSet(encoded string) (*descriptorpb.FileDescriptorSet, error) {
	decodedContents, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	var fds descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(decodedContents, &fds); err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return &fds, nil
}

func makeFiledescriptorSet(transcoders []transcoderWithCluster) (*descriptorpb.FileDescriptorSet, error) {
	var files []*descriptorpb.FileDescriptorProto
	var errors []error
	for _, t := range transcoders {
		fileDescriptor, err := decodeBase64FiledescriptorSet(t.transcoder.Spec.EncodedProtoDescriptor)
		if err != nil {
			errors = append(errors, err)
		} else {
			files = append(files, fileDescriptor.File...)
		}
	}
	if len(errors) > 0 {
		return nil, multierr.New(errors...)
	}
	return &descriptorpb.FileDescriptorSet{
		File: files,
	}, nil
}

func makeRoute(clusterName string, transcoderSpec HttpGrpcTranscoderSpec) *route.Route {
	return &route.Route{
		Name: "route-" + clusterName,
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{
				Prefix: "/" + transcoderSpec.FullyQualifiedProtoServiceName,
			},
		},
		Action: &route.Route_Route{
			Route: &route.RouteAction{
				ClusterSpecifier: &route.RouteAction_Cluster{
					Cluster: clusterName,
				},
			},
		},
	}
}

func makeHTTPListener(httpConfig httpListenerConfig, transcoders []transcoderWithCluster) (*listener.Listener, error) {
	var serviceNames []string
	var routes []*route.Route

	for _, t := range transcoders {
		serviceNames = append(serviceNames, t.transcoder.Spec.FullyQualifiedProtoServiceName)
		routes = append(routes, makeRoute(t.clusterName, t.transcoder.Spec))
	}

	fds, err := makeFiledescriptorSet(transcoders)
	if err != nil {
		return nil, fnerrors.InternalError("failed to created an aggregated FiledescriptorSet: %w", err)
	}
	bytes, err := proto.Marshal(fds)
	if err != nil {
		return nil, fnerrors.InternalError("failed to marshal the FiledescriptorSet: %w", err)
	}
	transcoderPb := &grpcjsontranscoder.GrpcJsonTranscoder{
		Services:    serviceNames,
		AutoMapping: true,
		DescriptorSet: &grpcjsontranscoder.GrpcJsonTranscoder_ProtoDescriptorBin{
			ProtoDescriptorBin: bytes,
		},
	}
	transcoderpbst, err := anypb.New(transcoderPb)
	if err != nil {
		return nil, fnerrors.BadInputError("failed to create the transcoder anypb: %w", err)
	}
	routerconfig, err := anypb.New(&routerfilter.Router{})
	if err != nil {
		return nil, fnerrors.BadInputError("failed to create the routerconfig anypb: %w", err)
	}
	fileAccessLog, err := anypb.New(&filev3.FileAccessLog{Path: "/dev/stdout"})
	if err != nil {
		return nil, fnerrors.BadInputError("failed to create fileaccesslog anypb: %w", err)
	}

	// HTTP filter configuration
	manager := &hcm.HttpConnectionManager{
		CodecType:  hcm.HttpConnectionManager_AUTO,
		StatPrefix: StatPrefix,
		AccessLog: []*accesslogv3.AccessLog{{
			Name: wellknown.FileAccessLog,
			ConfigType: &accesslogv3.AccessLog_TypedConfig{
				TypedConfig: fileAccessLog,
			},
		}},
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: &route.RouteConfiguration{
				Name: LocalRouteName,
				VirtualHosts: []*route.VirtualHost{{
					Name:    LocalVirtualHost,
					Domains: []string{"*"},
					Routes:  routes,
				},
				}}},
		HttpFilters: []*hcm.HttpFilter{{
			Name: wellknown.GRPCJSONTranscoder,
			ConfigType: &hcm.HttpFilter_TypedConfig{
				TypedConfig: transcoderpbst,
			},
		}, {
			Name: wellknown.Router,
			ConfigType: &hcm.HttpFilter_TypedConfig{
				TypedConfig: routerconfig,
			},
		}},
	}

	pbst, err := anypb.New(manager)
	if err != nil {
		return nil, err
	}

	return &listener.Listener{
		Name: httpConfig.name,
		Address: &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Protocol: core.SocketAddress_TCP,
					Address:  httpConfig.address,
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: httpConfig.port,
					},
				},
			},
		},
		FilterChains: []*listener.FilterChain{{
			Filters: []*listener.Filter{{
				Name: wellknown.HTTPConnectionManager,
				ConfigType: &listener.Filter_TypedConfig{
					TypedConfig: pbst,
				},
			}},
		}},
	}, nil
}
