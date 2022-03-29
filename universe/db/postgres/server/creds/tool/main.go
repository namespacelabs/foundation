// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"

	"namespacelabs.dev/foundation/provision/tool/bootstrap"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/std/secrets"
)

type tool struct{}

func main() {
	bootstrap.RunTool(tool{})
}

func getSecrets(devMap *secrets.SecretDevMap) []*secrets.SecretDevMap_SecretSpec {
	for _, conf := range devMap.Configure {
		if conf.PackageName == "namespacelabs.dev/foundation/universe/db/postgres/creds" {
			return conf.Secret
		}
	}
	return nil
}

func (tool) Apply(ctx context.Context, r bootstrap.Request, out *bootstrap.ApplyOutput) error {
	devMap, _, err := secrets.CollectSecrets(ctx, r.Focus.Server, nil)
	if err != nil {
		return err
	}

	for _, secret := range getSecrets(devMap) {
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

func (tool) Delete(ctx context.Context, r bootstrap.Request, out *bootstrap.DeleteOutput) error {
	return nil
}