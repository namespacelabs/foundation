// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	envoy_config_bootstrap_v3 "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	envoy_config_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_config_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_config_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"

	"google.golang.org/protobuf/types/known/durationpb"
)

type Bootstrap = envoy_config_bootstrap_v3.Bootstrap

type Address = envoy_config_core_v3.Address

type APIVersion = envoy_config_core_v3.ApiVersion

const (
	ApiVersion_V3 APIVersion = envoy_config_core_v3.ApiVersion_V3
)

// Option ...
type Option func(*Bootstrap, map[string]string)

// NodeID sets the envoy node ID.
func NodeID(s string) Option {
	return func(b *Bootstrap, _ map[string]string) {
		b.Node.Id = s
	}
}

// NodeCluster sets the envoy node Cluster name.
func NodeCluster(s string) Option {
	return func(b *Bootstrap, _ map[string]string) {
		b.Node.Cluster = s
	}
}

// ResourceVersion sets the default resource API version Envoy will ask for.
func ResourceVersion(vers APIVersion) Option {
	return func(b *Bootstrap, _ map[string]string) {
		b.DynamicResources.LdsConfig.ResourceApiVersion = vers
		b.DynamicResources.CdsConfig.ResourceApiVersion = vers
	}
}

// SetNodeOnFirstMessageOnly tells Envoy to only send the Node message once.
func SetNodeOnFirstMessageOnly(value bool) Option {
	return func(b *Bootstrap, _ map[string]string) {
		b.DynamicResources.AdsConfig.SetNodeOnFirstMessageOnly = value
	}
}

// AdminAddress sets the address the admin server will listen on.
func AdminAddress(addr *Address) Option {
	return func(b *Bootstrap, _ map[string]string) {
		b.Admin.Address = addr
	}
}

// ManagementAddress sets the address to connect to the xDS management server.
func ManagementAddress(addr *Address) Option {
	return func(b *Bootstrap, ctx map[string]string) {
		xdsName := ctx["xds-name"]

		for _, c := range b.StaticResources.Clusters {
			// Find the xDS cluster.
			if c.LoadAssignment.ClusterName != xdsName {
				continue
			}

			// Stuff the address into a host endpoint.
			ep := &envoy_config_endpoint_v3.LbEndpoint{
				HostIdentifier: &envoy_config_endpoint_v3.LbEndpoint_Endpoint{
					Endpoint: &envoy_config_endpoint_v3.Endpoint{
						Address: addr,
					},
				},
			}

			// Add  the endpoint to the cluster.
			c.LoadAssignment.Endpoints[0].LbEndpoints = append(
				c.LoadAssignment.Endpoints[0].LbEndpoints, ep,
			)
		}
	}
}

// ManagementClusterName sets the name of the xDS management cluster.
// This is the name that must be subsequently used to build ConfigSource
// messages in Envoy resources.
func ManagementClusterName(name string) Option {
	return func(b *Bootstrap, ctx map[string]string) {
		oldName := ctx["xds-name"]

		for _, c := range b.StaticResources.Clusters {
			if c.Name == oldName {
				c.Name = name
				c.LoadAssignment.ClusterName = name
			}
		}

		for _, g := range b.DynamicResources.GetAdsConfig().GetGrpcServices() {
			if g.GetEnvoyGrpc().GetClusterName() == oldName {
				g.GetEnvoyGrpc().ClusterName = name
			}
		}

		ctx["xds-name"] = name
	}
}

// New returns a new bootstrap protobuf.
func New(options ...Option) (*Bootstrap, error) {
	type Admin = envoy_config_bootstrap_v3.Admin                                 //nolint
	type ApiConfigSource = envoy_config_core_v3.ApiConfigSource                  //nolint
	type Cluster = envoy_config_cluster_v3.Cluster                               //nolint
	type ConfigSource = envoy_config_core_v3.ConfigSource                        //nolint
	type DynamicResources = envoy_config_bootstrap_v3.Bootstrap_DynamicResources //nolint
	type GrpcService = envoy_config_core_v3.GrpcService                          //nolint
	type Node = envoy_config_core_v3.Node                                        //nolint
	type StaticResources = envoy_config_bootstrap_v3.Bootstrap_StaticResources   //nolint

	// Used to keep track of the original xDS cluster name if modified as an option.
	ctx := map[string]string{
		"xds-name": "xds_cluster",
	}

	b := &Bootstrap{
		Node: &Node{
			Id:       "",
			Cluster:  "",
			Metadata: nil,
			Locality: nil,
		},
		Admin: &Admin{
			ProfilePath:   "",
			Address:       nil,
			SocketOptions: nil,
		},
		StaticResources: &StaticResources{
			Listeners: nil,
			Clusters: []*Cluster{
				{
					Name:                 ctx["xds-name"],
					ConnectTimeout:       durationpb.New(5 * time.Second),
					Http2ProtocolOptions: &envoy_config_core_v3.Http2ProtocolOptions{},
					ClusterDiscoveryType: &envoy_config_cluster_v3.Cluster_Type{
						Type: envoy_config_cluster_v3.Cluster_STATIC,
					},
					LoadAssignment: &envoy_config_endpoint_v3.ClusterLoadAssignment{
						ClusterName: ctx["xds-name"],
						Endpoints: []*envoy_config_endpoint_v3.LocalityLbEndpoints{
							{
								LbEndpoints: []*envoy_config_endpoint_v3.LbEndpoint{},
							}},
					},
				},
			},
		},
		DynamicResources: &DynamicResources{
			CdsConfig: &ConfigSource{
				ResourceApiVersion: ApiVersion_V3,
				ConfigSourceSpecifier: &envoy_config_core_v3.ConfigSource_Ads{
					Ads: &envoy_config_core_v3.AggregatedConfigSource{},
				},
			},
			LdsConfig: &ConfigSource{
				ResourceApiVersion: ApiVersion_V3,
				ConfigSourceSpecifier: &envoy_config_core_v3.ConfigSource_Ads{
					Ads: &envoy_config_core_v3.AggregatedConfigSource{},
				},
			},
			AdsConfig: &ApiConfigSource{
				ApiType:                   envoy_config_core_v3.ApiConfigSource_GRPC,
				TransportApiVersion:       ApiVersion_V3,
				RefreshDelay:              nil,
				RequestTimeout:            nil,
				RateLimitSettings:         nil,
				SetNodeOnFirstMessageOnly: false,
				GrpcServices: []*GrpcService{
					{
						TargetSpecifier: &envoy_config_core_v3.GrpcService_EnvoyGrpc_{
							EnvoyGrpc: &envoy_config_core_v3.GrpcService_EnvoyGrpc{
								ClusterName: ctx["xds-name"],
							},
						},
					},
				},
			},
		},
		ClusterManager:             nil,
		HdsConfig:                  nil,
		FlagsPath:                  "",
		StatsSinks:                 nil,
		StatsConfig:                nil,
		StatsFlushInterval:         nil,
		Watchdog:                   nil,
		Tracing:                    nil,
		LayeredRuntime:             nil,
		OverloadManager:            nil,
		EnableDispatcherStats:      false,
		StatsServerVersionOverride: nil,
		UseTcpForDnsLookups:        false,
	}

	for _, o := range options {
		o(b, ctx)
	}

	if err := b.Validate(); err != nil {
		return nil, err
	}

	return b, nil
}

// NewAddress parses the addr string into a Envoy Address that can
// subsequently be used in an Option. If the address contains ":", it
// is assumed to be a socket "address:port" spec, otherwise is it the
// path to a pipe.
func NewAddress(addr string) (*Address, error) {
	address := Address{}

	if strings.Contains(addr, ":") {
		parts := strings.SplitN(addr, ":", 2)

		addr := parts[0]
		if addr == "" {
			addr = "0.0.0.0"
		}

		port, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid socket address %q: %w", addr, err)
		}

		address.Address = &envoy_config_core_v3.Address_SocketAddress{
			SocketAddress: &envoy_config_core_v3.SocketAddress{
				Address: addr,
				PortSpecifier: &envoy_config_core_v3.SocketAddress_PortValue{
					PortValue: uint32(port),
				},
			},
		}
	} else {
		address.Address = &envoy_config_core_v3.Address_Pipe{
			Pipe: &envoy_config_core_v3.Pipe{
				Path: addr,
				Mode: 0640,
			},
		}
	}

	return &address, nil
}
