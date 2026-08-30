// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package servercore

import (
	"testing"

	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/std/go/core"
)

type registrarTestService interface{}

func TestGRPCRegistrarRegistersServiceOnNamedListener(t *testing.T) {
	defaultServer := grpc.NewServer()
	namedServer := grpc.NewServer()
	server := &ServerImpl{
		srv: map[string][]*grpc.Server{
			"":        {defaultServer},
			"private": {namedServer},
		},
	}
	registrar := server.Scope(&core.Package{PackageName: "test"})

	desc := &grpc.ServiceDesc{
		ServiceName: "test.Service",
		HandlerType: (*registrarTestService)(nil),
	}
	implementation := struct{}{}

	registrar.RegisterService(desc, implementation)
	registrar.GRPCRegistrar("private").RegisterService(desc, implementation)

	if _, ok := defaultServer.GetServiceInfo()[desc.ServiceName]; !ok {
		t.Fatal("service was not registered on the default listener")
	}
	if _, ok := namedServer.GetServiceInfo()[desc.ServiceName]; !ok {
		t.Fatal("service was not registered on the named listener")
	}
}
