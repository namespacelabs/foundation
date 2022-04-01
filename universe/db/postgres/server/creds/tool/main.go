// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"

	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/std/secrets"
)

type tool struct{}

func main() {
	configure.RunTool(tool{})
}

func (tool) Apply(ctx context.Context, r configure.Request, out *configure.ApplyOutput) error {
	collection, err := secrets.Collect(r.Focus.Server)
	if err != nil {
		return err
	}

	for _, secret := range collection.SecretsOf("namespacelabs.dev/foundation/universe/db/postgres/creds") {
		switch secret.Name {
		case "postgres_user_file":
			out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
				With: &kubedef.ContainerExtension{
					Env: []*kubedef.ContainerExtension_Env{{
						Name:  "POSTGRES_USER_FILE",
						Value: secret.FromPath,
					}},
				}})
		case "postgres_password_file":
			out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
				With: &kubedef.ContainerExtension{
					Env: []*kubedef.ContainerExtension_Env{{
						Name:  "POSTGRES_PASSWORD_FILE",
						Value: secret.FromPath,
					}},
				}})
		default:
		}
	}

	return nil
}

func (tool) Delete(ctx context.Context, r configure.Request, out *configure.DeleteOutput) error {
	return nil
}
