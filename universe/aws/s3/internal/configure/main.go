// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/framework/provisioning"
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

func (tool) Apply(ctx context.Context, r provisioning.StackRequest, out *provisioning.ApplyOutput) error {
	col, err := secrets.Collect(r.Focus.Server)
	if err != nil {
		return err
	}

	// TODO: creds should be definable per db instance #217
	args := []string{}
	for _, secret := range col.SecretsOf("namespacelabs.dev/foundation/universe/aws/client") {
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

	out.ServerExtensions = append(out.ServerExtensions, &schema.ServerExtension{
		ExtendContainer: []*schema.ContainerExtension{
			{BinaryRef: schema.MakePackageSingleRef(initContainerToConfigure), Args: args},
		},
	})

	return nil
}

func (tool) Delete(ctx context.Context, r provisioning.StackRequest, out *provisioning.DeleteOutput) error {
	return nil
}
