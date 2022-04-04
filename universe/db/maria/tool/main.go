// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/std/secrets"
)

type tool struct{}

func main() {
	configure.RunTool(tool{})
}

func getSecrets(devMap *secrets.SecretDevMap) []*secrets.SecretDevMap_SecretSpec {
	for _, conf := range devMap.Configure {
		if conf.PackageName == "namespacelabs.dev/foundation/universe/db/maria/creds" {
			return conf.Secret
		}
	}
	return nil
}

func (tool) Apply(ctx context.Context, r configure.Request, out *configure.ApplyOutput) error {
	args := []string{}

	devMap, _, err := secrets.CollectSecrets(ctx, r.Focus.Server, nil)
	if err != nil {
		return err
	}

	// TODO: creds should be definable per db instance #217
	for _, secret := range getSecrets(devMap) {
		switch secret.Name {
		case "mariadb_password_file":
			args = append(args, fmt.Sprintf("--mariadb_password_file=%s", secret.FromPath))
		default:
		}
	}

	out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
		With: &kubedef.ContainerExtension{
			InitContainer: []*kubedef.ContainerExtension_InitContainer{{
				PackageName: "namespacelabs.dev/foundation/universe/db/maria/init",
				Arg:         args,
			}},
		}})

	return nil
}

func (tool) Delete(ctx context.Context, r configure.Request, out *configure.DeleteOutput) error {
	return nil
}
