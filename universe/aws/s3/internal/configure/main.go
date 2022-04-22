// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/golang/protobuf/proto"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/universe/aws/s3"
)

const (
	// Bucket type name provided by the aws/s3 package.
	bucketTypeName           = "Bucket"
	initContainerToConfigure = "namespacelabs.dev/foundation/universe/aws/s3/internal/managebuckets/init"
)

type tool struct{}

func main() {
	configure.RunTool(tool{})
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

func (tool) Apply(ctx context.Context, r configure.StackRequest, out *configure.ApplyOutput) error {
	if r.Env.Runtime != "kubernetes" {
		return nil
	}
	col, err := secrets.Collect(r.Focus.Server)
	if err != nil {
		return err
	}

	// TODO: creds should be definable per db instance #217
	args := []string{}
	for _, secret := range col.SecretsOf("namespacelabs.dev/foundation/universe/aws/s3") {
		switch secret.Name {
		case "aws_credentials_file":
			args = append(args, fmt.Sprintf("--aws_credentials_file=%s", secret.FromPath))
		default:
		}
	}

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
