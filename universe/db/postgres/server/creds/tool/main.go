// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"

	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/secrets"
)

type tool struct{}

func main() {
	h := configure.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(tool{})
	configure.Handle(h)
}

func (tool) Apply(ctx context.Context, r configure.StackRequest, out *configure.ApplyOutput) error {
	collection, err := secrets.Collect(r.Focus.Server)
	if err != nil {
		return err
	}

	for _, secret := range collection.SecretsOf("namespacelabs.dev/foundation/universe/db/postgres/internal/gencreds") {
		switch secret.Name {
		case "postgres-password-file":
			out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
				With: &kubedef.ContainerExtension{
					Env: []*schema.BinaryConfig_EnvEntry{{
						Name:  "POSTGRES_PASSWORD_FILE",
						Value: secret.FromPath,
					}},
				}})
		default:
		}
	}

	return nil
}

func (tool) Delete(ctx context.Context, r configure.StackRequest, out *configure.DeleteOutput) error {
	return nil
}
