// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/universe/storage/minio/s3"
)

const (
	// Bucket type name provided by the aws/s3 package.
	bucketTypeName           = "Bucket"
	packageName    = "namespacelabs.dev/foundation/universe/storage/minio"
	serviceName    = "api"
	initContainerToConfigure = "namespacelabs.dev/foundation/universe/storage/minio/s3/internal/managebuckets/init"
)

type tool struct{}

func main() {
	h := configure.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(tool{})
	configure.Handle(h)
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

func getEndpoint(s *schema.Stack) string {
	for _, e := range s.Endpoint {
		if e.ServiceName == serviceName && e.ServerOwner == packageName {
			return fmt.Sprintf("http://%s:%d", e.AllocatedName, e.Port.ContainerPort)
		}
	}
	return ""
}

func (tool) Apply(ctx context.Context, r configure.StackRequest, out *configure.ApplyOutput) error {
	endpoint := getEndpoint(r.Stack)
	if endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	args := []string{fmt.Sprintf("--init_endpoint=%s", endpoint)}

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

func (tool) Delete(ctx context.Context, r configure.StackRequest, out *configure.DeleteOutput) error {
	return nil
}
