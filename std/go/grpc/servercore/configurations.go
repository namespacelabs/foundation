// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package servercore

import (
	"context"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"namespacelabs.dev/foundation/std/go/core"
)

var configConfigs = map[string]ListenerConfiguration{}

type DefaultConfiguration struct{}
type DefaultConfigurationWithSharedMtls struct{}

type ListenerConfiguration interface {
	CreateListener(context.Context, string, ListenOpts) (net.Listener, error)
}

type GrpcListenerConfiguration interface {
	ListenerConfiguration
	TransportCredentials(string) credentials.TransportCredentials
	ServerOpts(string) []grpc.ServerOption
}

type SharedMtlsGrpcListenerConfiguration interface {
	ListenerConfiguration

	UseFoundationMTLSConfiguration()
	ServerOpts(string) []grpc.ServerOption
}

func SetListenerConfiguration(name string, conf ListenerConfiguration) {
	core.AssertNotRunning("SetServiceConfiguration")

	if _, ok := configConfigs[name]; ok {
		panic("configuration already set")
	}

	configConfigs[name] = conf
}

func SetGrpcListenerConfiguration(name string, conf GrpcListenerConfiguration) {
	SetListenerConfiguration(name, conf)
}

func listenerConfiguration(name string) ListenerConfiguration {
	return configConfigs[name]
}

func (DefaultConfiguration) CreateListener(ctx context.Context, name string, opts ListenOpts) (net.Listener, error) {
	return opts.CreateNamedListener(ctx, name)
}

func (DefaultConfigurationWithSharedMtls) UseFoundationMTLSConfiguration() {}
