// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package grpc

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"namespacelabs.dev/foundation/std/go/core"
)

var configConfigs = map[string]Configuration{}

type Configuration interface {
	TransportCredentials() credentials.TransportCredentials
	ServerOpts() []grpc.ServerOption
}

func SetServiceConfiguration(name string, conf Configuration) {
	core.AssertNotRunning("grpc.SetConfiguration")

	if _, ok := configConfigs[name]; ok {
		panic("configuration already set")
	}

	configConfigs[name] = conf
}

func ServiceConfiguration(name string) Configuration {
	return configConfigs[name]
}
