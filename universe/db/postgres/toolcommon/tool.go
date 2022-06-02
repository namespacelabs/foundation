// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package toolcommon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/types"
	"namespacelabs.dev/foundation/universe/db/postgres"
)

const (
	id       = "init.postgres.foundation.namespacelabs.dev"
	basePath = "/postgres/init"
)

func makeKey(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func mountConfigs(dbMap map[schema.PackageName][]*postgres.Database, namespace string, name string, out *configure.ApplyOutput) ([]string, error) {
	args := []string{}

	data := map[string]string{}
	items := []*kubedef.SpecExtension_Volume_ConfigMap_Item{}

	mountPath := filepath.Join(basePath, name)

	for packageName, dbs := range dbMap {
		for _, db := range dbs {
			schemaPath := filepath.Join(packageName.String(), "schema", db.SchemaFile.Path)
			schemaKey := makeKey(schemaPath)

			data[schemaKey] = string(db.SchemaFile.Contents)
			items = append(items, &kubedef.SpecExtension_Volume_ConfigMap_Item{
				Key:  schemaKey,
				Path: schemaPath,
			})

			configPath := filepath.Join(packageName.String(), "config", db.Name)
			configKey := makeKey(configPath)

			config := &postgres.Database{
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

	configMapName := fmt.Sprintf("%s.%s", name, id)

	out.Definitions = append(out.Definitions, kubedef.Apply{
		Description: "Postgres Init ConfigMap",
		Resource:    "configmaps",
		Namespace:   namespace,
		Name:        configMapName,
		Body:        corev1.ConfigMap(configMapName, namespace).WithData(data),
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

func Apply(ctx context.Context, r configure.StackRequest, dbs map[schema.PackageName][]*postgres.Database, name string, out *configure.ApplyOutput) error {
	if r.Env.Runtime != "kubernetes" {
		return nil
	}

	namespace := kubetool.FromRequest(r).Namespace

	args, err := mountConfigs(dbs, namespace, name, out)
	if err != nil {
		return err
	}

	out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
		With: &kubedef.ContainerExtension{
			InitContainer: []*kubedef.ContainerExtension_InitContainer{{
				PackageName: "namespacelabs.dev/foundation/universe/db/postgres/init",
				Arg:         args,
			}},
		}})

	return nil
}

func Delete(r configure.StackRequest, name string, out *configure.DeleteOutput) error {
	if r.Env.Runtime != "kubernetes" {
		return nil
	}

	namespace := kubetool.FromRequest(r).Namespace

	out.Ops = append(out.Ops, kubedef.Delete{
		Description: "Postgres Delete ConfigMap",
		Resource:    "configmaps",
		Namespace:   namespace,
		Name:        fmt.Sprintf("%s.%s", name, id),
	})

	return nil
}
