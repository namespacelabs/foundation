// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/std/secrets"
)

const (
	self             = "namespacelabs.dev/foundation/universe/storage/s3/internal/prepare"
	initContainer    = "namespacelabs.dev/foundation/universe/storage/s3/internal/managebuckets"
	localstackServer = "namespacelabs.dev/foundation/universe/development/localstack"
	minioServer      = "namespacelabs.dev/foundation/universe/storage/minio/server"
	s3node           = "namespacelabs.dev/foundation/universe/storage/s3"

	useLocalstackFlag = "storage_s3_localstack_endpoint"
	useMinioFlag      = "storage_s3_minio_endpoint"
	serializedFlag    = "storage_s3_configured_buckets_protojson"

	localstackEndpoint = "api"
	minioEndpoint      = "api"
)

func main() {
	if err := configure.RunServer(context.Background(), func(sr grpc.ServiceRegistrar) {
		h := configure.NewHandlers()
		h.Any().HandleStack(provisionHook{})

		protocol.RegisterInvocationServiceServer(sr, h.ServiceHandler())
	}); err != nil {
		log.Fatal(err)
	}
}

type provisionHook struct{}

func (provisionHook) Apply(ctx context.Context, req configure.StackRequest, out *configure.ApplyOutput) error {
	fmt.Fprintf(os.Stderr, "req focus server: %s\n", proto.MarshalTextString(req.Focus.Server))
	col, err := secrets.Collect(req.Focus.Server)
	fmt.Fprintf(os.Stderr, "col: %+v\n", col.DevMap.Configure)
	if err != nil {
		return err
	}
	//initArgs := []string{}
	envVars := []*kubedef.ContainerExtension_Env{}

	for _, secret := range col.SecretsOf("namespacelabs.dev/foundation/universe/storage/minio/configure") {
		if secret.Name == "secret_key" {
			envVars = append(envVars, &kubedef.ContainerExtension_Env{
				Name:  "MINIO_ROOT_PASSWORD_FILE",
				Value: secret.FromPath,
			})
		} else if secret.Name == "access_token" {
			envVars = append(envVars, &kubedef.ContainerExtension_Env{
				Name:  "MINIO_ROOT_USER_FILE",
				Value: secret.FromPath,
			})
		}
	}

	out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
		With: &kubedef.ContainerExtension{Env: envVars}})

	// fmt.Fprintf(os.Stderr, "entry.Server: %v\n", len(req.Stack.Entry))
	// for _, entry := range req.Stack.Entry {
	// 	fmt.Fprintf(os.Stderr, "entry.Server: %v\n", entry.Server)
	// }

	return nil
	// systemInfo := &kubedef.SystemInfo{}
	// if err := req.UnpackInput(systemInfo); err != nil {
	// 	return err
	// }

	// eksDetails := &eks.EKSServerDetails{}
	// if ok, err := req.CheckUnpackInput(eksDetails); err != nil {
	// 	return err
	// } else if !ok {
	// 	eksDetails = nil
	// }

	// buckets := map[string]*s3.BucketArgs{}
	// if err := configure.VisitAllocs(req.Focus.Server.Allocation, s3node, &s3.BucketArgs{},
	// 	func(alloc *schema.Allocation_Instance, instantiate *schema.Instantiate, args *s3.BucketArgs) error {
	// 		if existing, ok := buckets[args.GetBucketName()]; ok {
	// 			if !proto.Equal(existing, args) {
	// 				return fnerrors.UserError(nil, "%s: incompatible s3 bucket definitions for %q", alloc.InstanceOwner, args.GetBucketName())
	// 			}
	// 		} else {
	// 			buckets[args.GetBucketName()] = args
	// 		}
	// 		return nil
	// 	}); err != nil {
	// 	return err
	// }

	// var orderedBuckets []*s3.BucketArgs
	// for _, bucket := range buckets {
	// 	orderedBuckets = append(orderedBuckets, bucket)
	// }

	// sort.Slice(orderedBuckets, func(i, j int) bool {
	// 	return strings.Compare(orderedBuckets[i].GetBucketName(), orderedBuckets[j].GetBucketName()) < 0
	// })

	// if useLocalstack(req.Env) || useMinio(req.Env) {
	// 	for _, bucket := range orderedBuckets {
	// 		if region := bucket.GetRegion(); region == "" {
	// 			bucket.Region = "us-east-1" // Default to us-east-1 for testing purposes with localstack.
	// 		}
	// 	}
	// } else {
	// 	for _, bucket := range orderedBuckets {
	// 		if region := bucket.GetRegion(); region == "" {
	// 			if l := len(systemInfo.Regions); l == 0 {
	// 				return fmt.Errorf("s3 bucket %q: no region specified, and not a aws deployment", bucket.BucketName)
	// 			} else if l > 1 {
	// 				return fmt.Errorf("s3 bucket %q: no region specified, and deployed to multiple regions, won't pick one (deployed to %s)",
	// 					bucket.BucketName, strings.Join(systemInfo.Regions, " "))
	// 			} else {
	// 				bucket.Region = systemInfo.Regions[0]
	// 			}
	// 		}
	// 	}
	// }

	// // XXX should be "can consume IAM policies bit".
	// if eksDetails != nil {
	// 	buckets := make([]string, len(orderedBuckets))
	// 	bucketsWildcard := make([]string, len(orderedBuckets))
	// 	for k, bucket := range orderedBuckets {
	// 		buckets[k] = fmt.Sprintf("arn:aws:s3:::%s", bucket.BucketName)
	// 		bucketsWildcard[k] = fmt.Sprintf("arn:aws:s3:::%s/*", bucket.BucketName)
	// 	}

	// 	// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_examples_s3_rw-bucket.html
	// 	policy := fniam.PolicyDocument{
	// 		Version: "2012-10-17",
	// 		Statement: []fniam.StatementEntry{
	// 			{
	// 				Effect:   "Allow",
	// 				Action:   []string{"s3:ListBucket"},
	// 				Resource: buckets,
	// 			},
	// 			{
	// 				Effect:   "Allow",
	// 				Action:   []string{"s3:*Object"},
	// 				Resource: bucketsWildcard,
	// 			},
	// 		},
	// 	}

	// 	policyBytes, err := json.Marshal(policy)
	// 	if err != nil {
	// 		return fnerrors.InternalError("failed to serialize policy: %w", err)
	// 	}

	// 	associate := &fniam.OpAssociatePolicy{
	// 		RoleName:   eksDetails.ComputedIamRoleName,
	// 		PolicyName: "fn-universe-storage-s3-bucket-access",
	// 		PolicyJson: string(policyBytes),
	// 	}

	// 	out.Definitions = append(out.Definitions, defs.Static("S3 IAM Bucket Access policy", associate))
	// }

	// serializedBuckets, err := protojson.Marshal(&s3.MultipleBucketArgs{Bucket: orderedBuckets})
	// if err != nil {
	// 	return err
	// }

	// col, err := secrets.Collect(req.Focus.Server)
	// if err != nil {
	// 	return err
	// }

	// var commonArgs, initArgs []string
	// if useLocalstack(req.Env) {
	// 	var localstackService string
	// 	for _, endpoint := range req.Stack.Endpoint {
	// 		if endpoint.EndpointOwner == localstackServer && endpoint.ServiceName == localstackEndpoint {
	// 			localstackService = "http://" + endpoint.Address()
	// 			break
	// 		}
	// 	}

	// 	if localstackService == "" {
	// 		return fmt.Errorf("localstack is required, but no endpoint is present that exports %q in %q",
	// 			localstackEndpoint, localstackServer)
	// 	}

	// 	commonArgs = append(commonArgs, fmt.Sprintf("--%s=%s", useLocalstackFlag, localstackService))
	// } else if useMinio(req.Env) {
	// 	var service string
	// 	for _, endpoint := range req.Stack.Endpoint {
	// 		if endpoint.EndpointOwner == minioServer && endpoint.ServiceName == minioEndpoint {
	// 			service = "http://" + endpoint.Address()
	// 			break
	// 		}
	// 	}

	// 	if service == "" {
	// 		return fmt.Errorf("minio is required, but no endpoint is present that exports %q in %q", minioEndpoint, service)
	// 	}
	// 	for _, secret := range col.SecretsOf("namespacelabs.dev/foundation/universe/aws/client") {
	// 		if secret.Name == "aws_credentials_file" {
	// 			initArgs = append(initArgs, fmt.Sprintf("--aws_credentials_file=%s", secret.FromPath))
	// 		}
	// 	}
	// 	commonArgs = append(commonArgs, fmt.Sprintf("--%s=%s", useMinioFlag, service))
	// } else {
	// 	for _, secret := range col.SecretsOf("namespacelabs.dev/foundation/universe/aws/client") {
	// 		if secret.Name == "aws_credentials_file" {
	// 			initArgs = append(initArgs, fmt.Sprintf("--aws_credentials_file=%s", secret.FromPath))
	// 		}
	// 	}
	// }

	// commonArgs = append(commonArgs, fmt.Sprintf("--%s=%s", serializedFlag, serializedBuckets))
	// initArgs = append(initArgs, commonArgs...)

	// out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
	// 	With: &kubedef.ContainerExtension{
	// 		Args: commonArgs,
	// 	},
	// })

	// out.Extensions = append(out.Extensions, kubedef.ExtendInitContainer{
	// 	With: &kubedef.InitContainerExtension{
	// 		PackageName: initContainer,
	// 		Args:        initArgs,
	// 	},
	// })

	// return nil
}

func (provisionHook) Delete(ctx context.Context, req configure.StackRequest, out *configure.DeleteOutput) error {
	// Nothing to do.
	return nil
}
