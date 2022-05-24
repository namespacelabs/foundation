// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"log"

	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/secrets"
)

const (
	configuringExtension = "namespacelabs.dev/foundation/universe/storage/minio/configure"

	accessTokenSecretName = "access_token"
	secretKeySecretName   = "secret_key"

	minioAcessTokenEnv = "MINIO_ROOT_USER_FILE"
	minioSecretKeyEnv  = "MINIO_ROOT_PASSWORD_FILE"
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

// Provides secret values for the minio server.
func (provisionHook) Apply(ctx context.Context, req configure.StackRequest, out *configure.ApplyOutput) error {
	col, err := secrets.Collect(req.Focus.Server)
	if err != nil {
		return err
	}
	envVars := []*schema.BinaryConfig_EnvEntry{}

	for _, secret := range col.SecretsOf(configuringExtension) {
		if secret.Name == secretKeySecretName {
			envVars = append(envVars, &schema.BinaryConfig_EnvEntry{
				Name:  minioSecretKeyEnv,
				Value: secret.FromPath,
			})
		} else if secret.Name == accessTokenSecretName {
			envVars = append(envVars, &schema.BinaryConfig_EnvEntry{
				Name:  minioAcessTokenEnv,
				Value: secret.FromPath,
			})
		}
	}

	out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
		With: &kubedef.ContainerExtension{Env: envVars}})

	return nil
}

func (provisionHook) Delete(ctx context.Context, req configure.StackRequest, out *configure.DeleteOutput) error {
	// Nothing to do.
	return nil
}
