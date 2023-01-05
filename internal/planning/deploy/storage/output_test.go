// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package storage

import (
	"fmt"
	"testing"

	"github.com/bradleyjkemp/cupaloy"
	"gotest.tools/assert"
	jsontesting "namespacelabs.dev/foundation/internal/planning/deploy/testing"
	"namespacelabs.dev/foundation/schema"
)

func TestToStorageNetworkPlan(t *testing.T) {
	assertNetworkPlan(t, "empty", "", &schema.Stack{}, nil, nil, nil)

	endpoint1 := &schema.Endpoint{
		Type:          schema.Endpoint_INTERNET_FACING,
		ServiceName:   "service1",
		EndpointOwner: "my/service1",
		Port:          &schema.Endpoint_Port{Name: "port1", ContainerPort: 1230},
		AllocatedName: "allocated1",
		ServerOwner:   "server1",
		ServiceLabel:  "label1",
		ServiceMetadata: []*schema.ServiceMetadata{
			{Kind: "", Protocol: ""},
			{Kind: "kind11", Protocol: "http"},
			{Kind: "kind12", Protocol: "grpc"},
		},
	}
	assertNetworkPlan(t, "basic", "localhost",
		&schema.Stack{
			Entry: []*schema.Stack_Entry{
				{Server: &schema.Server{PackageName: "server1"}},
				{Server: &schema.Server{PackageName: "server2"}},
			},
		},
		// Focused servers
		[]schema.PackageName{
			"server1",
			"server2",
		},
		[]*PortFwd{
			{Endpoint: &schema.Endpoint{}},
			{LocalPort: 123, Endpoint: endpoint1},
			{LocalPort: 234, Endpoint: &schema.Endpoint{
				Type:          schema.Endpoint_PRIVATE,
				ServiceName:   "service2",
				EndpointOwner: "my/service2",
				Port:          &schema.Endpoint_Port{Name: "port2", ContainerPort: 2340},
				AllocatedName: "allocated2",
				// Not present in the stack, not sure if this can happen.
				ServerOwner:  "server3",
				ServiceLabel: "label2",
			}},
		},
		[]*schema.IngressFragment{
			{Domain: &schema.Domain{}},
			{
				Name:  "fragment1",
				Owner: "owner1",
				Domain: &schema.Domain{
					Fqdn:    "domain1.example.com",
					Managed: schema.Domain_CLOUD_MANAGED,
				},
				DomainCertificate: &schema.Certificate{PrivateKey: []byte("__private__")},
				Endpoint:          endpoint1,
				Manager:           "manager1",
				HttpPath: []*schema.IngressFragment_IngressHttpPath{
					{},
					{Path: "/path1", Kind: "kind1", Owner: "owner1", Service: "service1",
						ServicePort: 1230},
					{Path: "/path2", Kind: "kind2", Owner: "owner2", Service: "service2",
						ServicePort: 2340},
				},
				GrpcService: []*schema.IngressFragment_IngressGrpcService{
					{},
					{GrpcService: "grpc1", Owner: "owner1", Service: "service1", Method: []string{"method1", "method2"},
						ServicePort: 1231},
					{GrpcService: "grpc2", Owner: "owner2", Service: "service2",
						ServicePort: 2341},
				},
			},
			{
				Name:  "fragment2",
				Owner: "owner2",
				Domain: &schema.Domain{
					Fqdn:    "domain2.example.com",
					Managed: schema.Domain_LOCAL_MANAGED,
				},
				Manager: "manager2",
			},
		})
}

func assertNetworkPlan(t *testing.T, snapshotName string, localHostname string, stack *schema.Stack, focus []schema.PackageName, portFwds []*PortFwd, ingressFragments []*schema.IngressFragment) {
	plan := ToStorageNetworkPlan(localHostname, stack, focus, portFwds, ingressFragments)

	json := jsontesting.StableProtoToJson(t, plan)

	assert.NilError(t, cupaloy.SnapshotMulti(fmt.Sprintf("%s.json", snapshotName), json))
}
