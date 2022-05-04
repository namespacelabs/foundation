// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/universe/storage/s3"
)

const (
	self             = "namespacelabs.dev/foundation/universe/storage/s3/internal/prepare"
	initContainer    = "namespacelabs.dev/foundation/universe/storage/s3/internal/manageinit"
	localstackServer = "namespacelabs.dev/foundation/universe/development/localstack"
	s3node           = "namespacelabs.dev/foundation/universe/storage/s3"

	useLocalstackFlag = "storage_s3_use_localstack"
	serializedFlag    = "storage_s3_configured_buckets_protojson"
)

func main() {
	if err := configure.RunServer(context.Background(), func(sr grpc.ServiceRegistrar) {
		h := configure.NewHandlers()
		h.Any().HandleStack(provisionHook{})

		protocol.RegisterPrepareServiceServer(sr, prepareHook{})
		protocol.RegisterInvocationServiceServer(sr, configure.ProtocolHandler{Handlers: configure.HandlerCompat{Tool: configure.HandlersHandler{Handlers: h}}})
	}); err != nil {
		log.Fatal(err)
	}
}

type prepareHook struct{}

func (prepareHook) Prepare(ctx context.Context, req *protocol.PrepareRequest) (*protocol.PrepareResponse, error) {
	resp := &protocol.PrepareResponse{
		PreparedProvisionPlan: &protocol.PreparedProvisionPlan{
			Provisioning: []*schema.Invocation{
				{Binary: self}, // Call me back.
			},
			Init: []*schema.SidecarContainer{
				{Binary: initContainer},
			},
		},
	}

	// In development or testing, use localstack.
	if useLocalstack(req.Env) {
		resp.PreparedProvisionPlan.DeclaredStack = append(resp.PreparedProvisionPlan.DeclaredStack, localstackServer)
	}

	return resp, nil
}

type provisionHook struct{}

func (provisionHook) Apply(ctx context.Context, req configure.StackRequest, out *configure.ApplyOutput) error {
	systemInfo := &kubedef.SystemInfo{}
	if err := req.UnpackInput(systemInfo); err != nil {
		return err
	}

	buckets := map[string]*s3.BucketArgs{}
	if err := configure.VisitAllocs(req.Focus.Server.Allocation, s3node, &s3.BucketArgs{},
		func(alloc *schema.Allocation_Instance, instantiate *schema.Instantiate, args *s3.BucketArgs) error {
			if existing, ok := buckets[args.GetBucketName()]; ok {
				if !proto.Equal(existing, args) {
					return fnerrors.UserError(nil, "%s: incompatible s3 bucket definitions for %q", alloc.InstanceOwner, args.GetBucketName())
				}
			} else {
				buckets[args.GetBucketName()] = args
			}
			return nil
		}); err != nil {
		return err
	}

	var orderedBuckets []*s3.BucketArgs
	for _, bucket := range buckets {
		orderedBuckets = append(orderedBuckets, bucket)
	}

	sort.Slice(orderedBuckets, func(i, j int) bool {
		return strings.Compare(orderedBuckets[i].GetBucketName(), orderedBuckets[j].GetBucketName()) < 0
	})

	if !useLocalstack(req.Env) {
		for _, bucket := range orderedBuckets {
			if region := bucket.GetRegion(); region == "" {
				if l := len(systemInfo.Regions); l == 0 {
					return fmt.Errorf("s3 bucket %q: no region specified, and not a aws deployment", bucket.BucketName)
				} else if l > 1 {
					return fmt.Errorf("s3 bucket %q: no region specified, and deployed to multiple regions, won't pick one (deployed to %s)",
						bucket.BucketName, strings.Join(systemInfo.Regions, " "))
				} else {
					bucket.Region = systemInfo.Regions[0]
				}
			}
		}
	}

	serializedBuckets, err := protojson.Marshal(&s3.MultipleBucketArgs{Bucket: orderedBuckets})
	if err != nil {
		return err
	}

	col, err := secrets.Collect(req.Focus.Server)
	if err != nil {
		return err
	}

	var serverArgs, initArgs []string
	if useLocalstack(req.Env) {
		serverArgs = append(serverArgs, fmt.Sprintf("--%s", useLocalstackFlag))
		initArgs = append(initArgs, fmt.Sprintf("--%s", useLocalstackFlag))
	} else {
		for _, secret := range col.SecretsOf("namespacelabs.dev/foundation/universe/aws/client") {
			if secret.Name == "aws_credentials_file" {
				initArgs = append(initArgs, fmt.Sprintf("--aws_credentials_file=%s", secret.FromPath))
			}
		}
	}

	out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
		For: req.Focus.GetPackageName(),
		With: &kubedef.ContainerExtension{
			Args: append(serverArgs, fmt.Sprintf("--%s=%s", serializedFlag, serializedBuckets)),
		},
	})

	out.Extensions = append(out.Extensions, kubedef.ExtendInitContainer{
		For: req.Focus.GetPackageName(),
		With: &kubedef.InitContainerExtension{
			PackageName: initContainer,
			Args:        append(initArgs, fmt.Sprintf("--%s=%s", serializedFlag, serializedBuckets)),
		},
	})

	return nil
}

func useLocalstack(env *schema.Environment) bool {
	return env.GetPurpose() == schema.Environment_DEVELOPMENT || env.GetPurpose() == schema.Environment_TESTING
}

func (provisionHook) Delete(ctx context.Context, req configure.StackRequest, out *configure.DeleteOutput) error {
	// Nothing to do.
	return nil
}
