// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package toolcommon

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/internal/support/naming"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/types"
	"namespacelabs.dev/foundation/universe/db/maria"
)

const (
	id       = "init.mariadb.fn"
	basePath = "/mariadb/init"
)

func mountConfigs(dbMap map[schema.PackageName][]*maria.Database, kr *kubetool.ContextualEnv, name string, focus string, out *configure.ApplyOutput) ([]string, error) {
	args := []string{}

	data := map[string]string{}
	items := []*kubedef.SpecExtension_Volume_ConfigMap_Item{}

	mountPath := filepath.Join(basePath, name)

	for packageName, dbs := range dbMap {
		for _, db := range dbs {
			schemaPath := filepath.Join(packageName.String(), "schema", db.SchemaFile.Path)
			schemaKey := naming.StableID(schemaPath)

			data[schemaKey] = string(db.SchemaFile.Contents)
			items = append(items, &kubedef.SpecExtension_Volume_ConfigMap_Item{
				Key:  schemaKey,
				Path: schemaPath,
			})

			configPath := filepath.Join(packageName.String(), "config", db.Name)
			configKey := naming.StableID(configPath)

			config := &maria.Database{
				Name: db.Name,
				SchemaFile: &types.Resource{
					Path: filepath.Join(mountPath, schemaPath),
				},
				HostedAt: db.HostedAt,
			}

			configBytes, err := json.Marshal(config)
			if err != nil {
				return nil, err
			}
			data[configKey] = string(configBytes)

			items = append(items, &kubedef.SpecExtension_Volume_ConfigMap_Item{
				Key:  configKey,
				Path: configPath,
			})

			args = append(args, filepath.Join(mountPath, configPath))
		}
	}

	configMapName := fmt.Sprintf("%s.%s.%s", focus, name, id)

	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description:  "MariaDB Init ConfigMap",
		SetNamespace: kr.CanSetNamespace,
		Resource:     corev1.ConfigMap(configMapName, kr.Namespace).WithData(data),
	})

	volumeName := strings.Replace(configMapName, ".", "-", -1)

	out.Extensions = append(out.Extensions, kubedef.ExtendSpec{
		With: &kubedef.SpecExtension{
			Volume: []*kubedef.SpecExtension_Volume{{
				Name: volumeName,
				VolumeType: &kubedef.SpecExtension_Volume_ConfigMap_{
					ConfigMap: &kubedef.SpecExtension_Volume_ConfigMap{
						Name: configMapName,
						Item: items,
					},
				},
			}},
		},
	})

	out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
		With: &kubedef.ContainerExtension{
			VolumeMount: []*kubedef.ContainerExtension_VolumeMount{{
				Name:        volumeName,
				ReadOnly:    true,
				MountPath:   mountPath,
				MountOnInit: true,
			}},
		},
	})

	return args, nil
}

func Apply(ctx context.Context, r configure.StackRequest, dbs map[schema.PackageName][]*maria.Database, name string, initArgs []string, out *configure.ApplyOutput) error {
	if r.Env.Runtime != "kubernetes" {
		return nil
	}

	kr, err := kubetool.FromRequest(r)
	if err != nil {
		return err
	}

	args, err := mountConfigs(dbs, kr, name, r.Focus.Server.Id, out)
	if err != nil {
		return err
	}

	initArgs = append(initArgs, args...)

	out.ServerExtensions = append(out.ServerExtensions, &schema.ServerExtension{
		ExtendContainer: []*schema.ContainerExtension{
			{BinaryRef: schema.MakePackageSingleRef("namespacelabs.dev/foundation/universe/db/maria/internal/init"), Args: initArgs},
		},
	})

	return nil

}

func Delete(r configure.StackRequest, name string, out *configure.DeleteOutput) error {
	if r.Env.Runtime != "kubernetes" {
		return nil
	}

	kr, err := kubetool.FromRequest(r)
	if err != nil {
		return err
	}

	out.Invocations = append(out.Invocations, kubedef.Delete{
		Description:  "MariaDB Init ConfigMap",
		Resource:     "configmaps",
		SetNamespace: kr.CanSetNamespace,
		Namespace:    kr.Namespace,
		Name:         fmt.Sprintf("%s.%s", name, id),
	})

	return nil
}
