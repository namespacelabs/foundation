// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/framework/provisioning"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/universe/development/localstack/s3"
)

const (
	// Bucket type name provided by the aws/s3 package.
	bucketTypeName           = "Bucket"
	localstackPackageName    = "namespacelabs.dev/foundation/universe/development/localstack"
	localstackServiceName    = "api"
	initContainerToConfigure = "namespacelabs.dev/foundation/universe/development/localstack/s3/internal/managebuckets/init"
)

type tool struct{}

func main() {
	h := provisioning.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(tool{})
	provisioning.Handle(h)
}

func collectBuckets(server *schema.Server, owner string) ([]*s3.BucketConfig, error) {
	buckets := []*s3.BucketConfig{}

	for _, alloc := range server.Allocation {
		for _, instance := range alloc.Instance {
			for _, instantiated := range instance.Instantiated {
				if instantiated.GetPackageName() == owner && instantiated.GetType() == bucketTypeName {
					bucket := &s3.BucketConfig{}
					if err := proto.Unmarshal(instantiated.Constructor.Value, bucket); err != nil {
						return nil, err
					}
					buckets = append(buckets, bucket)
				}
			}
		}
	}
	return buckets, nil
}

func getLocalstackEndpoint(s *schema.Stack) string {
	for _, e := range s.Endpoint {
		if e.ServiceName == localstackServiceName && e.ServerOwner == localstackPackageName {
			return fmt.Sprintf("http://%s:%d", e.AllocatedName, e.Port.ContainerPort)
		}
	}
	return ""
}

func (tool) Apply(ctx context.Context, r provisioning.StackRequest, out *provisioning.ApplyOutput) error {
	localstackEndpoint := getLocalstackEndpoint(r.Stack)
	if localstackEndpoint == "" {
		return fmt.Errorf("localstack endpoint is required")
	}
	args := []string{fmt.Sprintf("--init_localstack_endpoint=%s", localstackEndpoint)}

	bucketConfigs, err := collectBuckets(r.Focus.Server, r.PackageOwner())
	if err != nil {
		return err
	}
	// Append json-serialized bucket configs.
	for _, bucketConfig := range bucketConfigs {
		json, err := json.Marshal(bucketConfig)
		if err != nil {
			return nil
		}
		args = append(args, string(json))
	}

	out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
		With: &kubedef.ContainerExtension{
			InitContainer: []*kubedef.ContainerExtension_InitContainer{{
				PackageName: initContainerToConfigure,
				Arg:         args,
			}},
		}})
	return nil
}

func (tool) Delete(ctx context.Context, r provisioning.StackRequest, out *provisioning.DeleteOutput) error {
	return nil
}
