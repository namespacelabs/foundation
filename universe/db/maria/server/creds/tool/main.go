// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"

	"namespacelabs.dev/foundation/framework/provisioning"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/secrets"
)

type tool struct{}

func main() {
	h := provisioning.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(tool{})
	provisioning.Handle(h)
}

func (tool) Apply(ctx context.Context, r provisioning.StackRequest, out *provisioning.ApplyOutput) error {
	col, err := secrets.Collect(r.Focus.Server)
	if err != nil {
		return err
	}

	for _, secret := range col.SecretsOf("namespacelabs.dev/foundation/universe/db/maria/incluster/creds") {
		switch secret.Name {
		case "mariadb-password-file":
			out.ServerExtensions = append(out.ServerExtensions, &schema.ServerExtension{
				ExtendContainer: []*schema.ContainerExtension{
					{Env: []*schema.BinaryConfig_EnvEntry{{
						Name:  "MARIADB_ROOT_PASSWORD_FILE",
						Value: secret.FromPath,
					}}},
				},
			})

		default:
		}
	}

	return nil
}

func (tool) Delete(ctx context.Context, r provisioning.StackRequest, out *provisioning.DeleteOutput) error {
	return nil
}
