// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/universe/db/maria"
	"namespacelabs.dev/foundation/universe/db/maria/incluster"
	"namespacelabs.dev/foundation/universe/db/maria/toolcommon"
)

type tool struct{}

func main() {
	configure.RunTool(tool{})
}

func collectDatabases(server *schema.Server, owner string, internalEndpoint *schema.Endpoint) (map[schema.PackageName][]*maria.Database, error) {
	dbs := map[schema.PackageName][]*maria.Database{}
	for _, alloc := range server.Allocation {
		for _, instance := range alloc.Instance {
			for _, instantiate := range instance.Instantiated {
				if instantiate.GetPackageName() == owner && instantiate.GetType() == "Database" {
					in := &incluster.Database{}
					if err := proto.Unmarshal(instantiate.Constructor.Value, in); err != nil {
						return nil, err
					}

					db := &maria.Database{
						Name:       in.Name,
						SchemaFile: in.SchemaFile,
						HostedAt: &maria.Endpoint{
							Address: internalEndpoint.AllocatedName,
							Port:    uint32(internalEndpoint.Port.ContainerPort),
						},
					}

					dbs[schema.PackageName(instance.InstanceOwner)] = append(dbs[schema.PackageName(instance.InstanceOwner)], db)
				}
			}
		}
	}
	return dbs, nil
}

func internalEndpoint(s *schema.Stack) *schema.Endpoint {
	for _, e := range s.Endpoint {
		if e.ServiceName == "mariadb" && e.ServerOwner == "namespacelabs.dev/foundation/universe/db/maria/server" {
			return e
		}
	}
	return nil
}

func (tool) Apply(ctx context.Context, r configure.StackRequest, out *configure.ApplyOutput) error {
	if r.Env.Runtime != "kubernetes" {
		return nil
	}

	args := []string{}

	col, err := secrets.Collect(r.Focus.Server)
	if err != nil {
		return err
	}

	// TODO: creds should be definable per db instance #217
	for _, secret := range col.SecretsOf("namespacelabs.dev/foundation/universe/db/maria/incluster/creds") {
		switch secret.Name {
		case "mariadb-password-file":
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
	endpoint := internalEndpoint(r.Stack)

	value, err := json.Marshal(endpoint)
	if err != nil {
		return err
	}
	out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
		With: &kubedef.ContainerExtension{
			Arg: []*kubedef.ContainerExtension_Arg{{
				Name:  "mariadb_endpoint",
				Value: string(value),
			}},
		}})

	dbs, err := collectDatabases(r.Focus.Server, r.PackageOwner(), endpoint)
	if err != nil {
		return err
	}

	return toolcommon.Apply(ctx, r, dbs, "incluster", out)
}

func (tool) Delete(ctx context.Context, r configure.StackRequest, out *configure.DeleteOutput) error {
	return toolcommon.Delete(r, "incluster", out)
}
