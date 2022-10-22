// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"log"

	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/framework/kubernetes/kubetool"
	"namespacelabs.dev/foundation/framework/provisioning"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning/tool/protocol"
	"namespacelabs.dev/foundation/universe/aws/eks"
)

func main() {
	if err := provisioning.RunServer(context.Background(), func(sr grpc.ServiceRegistrar) {
		h := provisioning.NewHandlers()
		h.Any().HandleStack(provisionHook{})

		protocol.RegisterInvocationServiceServer(sr, h.ServiceHandler())
	}); err != nil {
		log.Fatal(err)
	}
}

type provisionHook struct{}

func (provisionHook) Apply(ctx context.Context, r provisioning.StackRequest, out *provisioning.ApplyOutput) error {
	if r.Env.Runtime != "kubernetes" {
		return fnerrors.BadInputError("universe/aws/irsa only supports kubernetes")
	}

	serviceAccount := &kubedef.ServiceAccountDetails{}
	if err := r.UnpackInput(serviceAccount); err != nil {
		return err
	}

	eksCluster := &eks.EKSCluster{}
	if ok, err := r.CheckUnpackInput(eksCluster); err != nil {
		return err
	} else if !ok {
		return nil
	}

	eksServerDetails := &eks.EKSServerDetails{}
	if err := r.UnpackInput(eksServerDetails); err != nil {
		return err
	}

	kr, err := kubetool.MustNamespace(r)
	if err != nil {
		return err
	}

	result, err := eks.PrepareIrsa(eksCluster, eksServerDetails.ComputedIamRoleName, kr.Namespace, serviceAccount.ServiceAccountName, r.Focus.Server)
	if err != nil {
		return err
	}

	out.Invocations = append(out.Invocations, result.Invocations...)
	out.Extensions = append(out.Extensions, result.Extensions...)

	return nil
}

func (provisionHook) Delete(ctx context.Context, r provisioning.StackRequest, out *provisioning.DeleteOutput) error {
	if r.Env.Runtime != "kubernetes" {
		return fnerrors.BadInputError("universe/aws/irsa only supports kubernetes")
	}

	return nil
}
