// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package servercore

import (
	"fmt"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/philopon/go-toposort"
	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
	"namespacelabs.dev/foundation/std/grpc/requestid"
)

func interceptorsAsOpts() []grpc.ServerOption {
	registrations := interceptors.ServerInterceptors()

	var rid requestid.Interceptor
	registrations = append(registrations, interceptors.Registered{
		Name:   "namespace-rid",
		After:  []string{"otel-tracing"},
		Unary:  rid.Unary,
		Stream: rid.Streaming,
	})

	graph := toposort.NewGraph(len(registrations))

	names := make([]string, len(registrations))
	index := map[string]int{}
	for k, reg := range registrations {
		name := reg.Name
		if name == "" {
			name = fmt.Sprintf("$interceptor_%d", k)
		}
		names[k] = name
		index[name] = k

		graph.AddNode(name)
	}

	for k, reg := range registrations {
		for _, after := range reg.After {
			if _, ok := index[after]; ok {
				graph.AddEdge(after, names[k])
			}
		}
	}

	sorted, ok := graph.Toposort()
	if !ok {
		panic("loop in interceptor order")
	}

	core.ZLog.Debug().Strs("interceptors", sorted).Send()

	var coreU []grpc.UnaryServerInterceptor
	var coreS []grpc.StreamServerInterceptor
	for _, key := range sorted {
		reg := registrations[index[key]]
		coreU = append(coreU, reg.Unary)
		coreS = append(coreS, reg.Stream)
	}

	return []grpc.ServerOption{
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(coreS...)),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(coreU...)),
	}
}
