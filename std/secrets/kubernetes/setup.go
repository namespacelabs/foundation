// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/framework/kubernetes/kubetool"
	"namespacelabs.dev/foundation/framework/provisioning"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/keys"
	fnsecrets "namespacelabs.dev/foundation/internal/secrets/localsecrets"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/std/types"
	"namespacelabs.dev/go-ids"
)

type tool struct{}

func main() {
	h := provisioning.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(tool{})
	provisioning.Handle(h)
}

func (tool) Apply(ctx context.Context, r provisioning.StackRequest, out *provisioning.ApplyOutput) error {
	kr, err := kubetool.FromRequest(r)
	if err != nil {
		return err
	}

	collection, err := secrets.Collect(r.Focus.Server)
	if err != nil {
		return err
	}

	data, err := fillData(ctx, r.Focus.Server, r.Env, collection, r)
	if err != nil {
		return err
	}

	devMapBytes, err := prototext.Marshal(collection.DevMap)
	if err != nil {
		return err
	}

	devMapBinaryBytes, err := proto.Marshal(collection.DevMap)
	if err != nil {
		return err
	}

	devMapJSON, err := protojson.Marshal(collection.DevMap)
	if err != nil {
		return err
	}

	if data == nil {
		data = map[string][]byte{}
	}

	// We do something funky here, we keep the mapping as a secret, so that
	// k8s also maps it to the container's namespace, and we can load it
	// using our regular mechanisms. Within that dev map, we map each
	// secret to other files mounted as well.
	data["map.textpb"] = devMapBytes

	// Servers use the binary serialized version.
	data["map.binarypb"] = devMapBinaryBytes

	// We also include a JSON version of the map to facilitiate JS-based uses.
	data["map.json"] = devMapJSON

	serverSecretName := secrets.ServerSecretName(r.Focus.Server)

	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description:  "server secrets",
		SetNamespace: kr.CanSetNamespace,
		Resource: applycorev1.
			Secret(serverSecretName, kr.Namespace).
			WithType(v1.SecretTypeOpaque).
			WithAnnotations(kubedef.MakeAnnotations(r.Env, r.Focus.GetPackageName())).
			WithLabels(kubedef.MakeLabels(r.Env, r.Focus.Server)).
			WithData(data),
	})

	volId := fmt.Sprintf("fn-secrets-%s", r.Focus.Server.Id)

	out.Extensions = append(out.Extensions, kubedef.ExtendSpec{
		With: &kubedef.SpecExtension{
			Volume: []*kubedef.SpecExtension_Volume{{
				Name: volId,
				VolumeType: &kubedef.SpecExtension_Volume_Secret_{
					Secret: &kubedef.SpecExtension_Volume_Secret{
						SecretName: serverSecretName,
					},
				},
			}},
		}})

	var envVars []*schema.BinaryConfig_EnvEntry
	for j, user := range collection.UserManaged {
		for k, x := range user {
			if x.ExperimentalMountAsEnvVar != "" {
				envVars = append(envVars, &schema.BinaryConfig_EnvEntry{
					Name:                   x.ExperimentalMountAsEnvVar,
					ExperimentalFromSecret: fmt.Sprintf("%s:%s", serverSecretName, collection.Names[j][k]),
				})
			}
		}
	}

	for _, gen := range collection.Generated {
		if gen.Secret.ExperimentalMountAsEnvVar != "" {
			envVars = append(envVars, &schema.BinaryConfig_EnvEntry{
				Name:                   gen.Secret.ExperimentalMountAsEnvVar,
				ExperimentalFromSecret: fmt.Sprintf("%s:%s", gen.ResourceName, gen.Secret.Name),
			})
		}
	}

	out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
		With: &kubedef.ContainerExtension{
			Env: envVars,
			VolumeMount: []*kubedef.ContainerExtension_VolumeMount{{
				Name:        volId,
				ReadOnly:    true,
				MountPath:   secrets.MountPath,
				MountOnInit: true, // Allow secret access during server initialization
			}},
		}})

	for _, gen := range collection.Generated {
		generatedName := gen.ResourceName
		volId := "fn-secret-" + gen.ID

		out.Extensions = append(out.Extensions, kubedef.ExtendSpec{
			With: &kubedef.SpecExtension{
				Volume: []*kubedef.SpecExtension_Volume{{
					Name: volId,
					VolumeType: &kubedef.SpecExtension_Volume_Secret_{
						Secret: &kubedef.SpecExtension_Volume_Secret{
							SecretName: generatedName,
						},
					},
				}},
			}})

		out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
			With: &kubedef.ContainerExtension{
				VolumeMount: []*kubedef.ContainerExtension_VolumeMount{{
					Name:        volId,
					ReadOnly:    true,
					MountPath:   gen.Path,
					MountOnInit: true, // Allow secret access during server initialization
				}},
			}})

		switch {
		case gen.Secret.InitializeWith != nil:
			src := &schema.SerializedInvocationSource{
				Description: "Generated server secrets",
			}

			create := &kubedef.OpCreateSecretConditionally{
				Namespace:         kr.Namespace,
				SetNamespace:      kr.CanSetNamespace,
				Name:              generatedName,
				UserSpecifiedName: gen.Secret.Name,
			}

			var err error
			src.Impl, err = anypb.New(create)
			if err != nil {
				return err
			}

			resource, err := anypb.New(&types.DeferredResource{FromInvocation: gen.Secret.InitializeWith})
			if err != nil {
				return err
			}

			src.Computable = append(src.Computable, &schema.SerializedInvocationSource_ComputableValue{
				Name:  "value",
				Value: resource,
			})

			out.InvocationSources = append(out.InvocationSources, src)

		case gen.Secret.SelfSignedTlsCertificate != nil:
			src := &schema.SerializedInvocationSource{
				Description: "Generated self-signed certificate bundle",
			}

			create := &kubedef.OpCreateSecretConditionally{
				Namespace:             kr.Namespace,
				SetNamespace:          kr.CanSetNamespace,
				Name:                  generatedName,
				UserSpecifiedName:     gen.Secret.Name,
				SelfSignedCertificate: gen.Secret.SelfSignedTlsCertificate,
			}

			var err error
			src.Impl, err = anypb.New(create)
			if err != nil {
				return err
			}

			out.InvocationSources = append(out.InvocationSources, src)

		default:
			data := map[string][]byte{}
			switch gen.Secret.Generate.Format {
			case secrets.GenerateSpecification_FORMAT_BASE32:
				data[gen.Secret.Name] = []byte(ids.NewRandomBase32ID(int(gen.Secret.Generate.RandomByteCount)))
			default: // Including BASE64
				raw := make([]byte, gen.Secret.Generate.RandomByteCount)
				_, _ = rand.Reader.Read(raw)
				data[gen.Secret.Name] = []byte(base64.RawStdEncoding.EncodeToString(raw))
			}

			out.Invocations = append(out.Invocations, kubedef.Create{
				Description:         "Generated server secrets",
				SetNamespace:        kr.CanSetNamespace,
				SkipIfAlreadyExists: true,
				Resource:            "secrets",
				Body: &v1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      generatedName,
						Namespace: kr.Namespace,
						Labels:    kubedef.MakeLabels(r.Env, nil),
					},
					Data: data,
				},
			})
		}
	}

	// The required secrets are then mounted to /secrets, where this extension can
	// pick them up. A map.textpb is also synthesized.
	if r.Focus.Server.Framework == schema.Framework_GO {
		out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
			With: &kubedef.ContainerExtension{
				Args: []string{
					"--server_secrets_basepath=/secrets/server",
				},
			},
		})
	}

	return nil
}

func (tool) Delete(ctx context.Context, r provisioning.StackRequest, out *provisioning.DeleteOutput) error {
	kr, err := kubetool.FromRequest(r)
	if err != nil {
		return err
	}

	out.Invocations = append(out.Invocations, kubedef.Delete{
		Description:  "server secrets",
		Resource:     "secrets",
		SetNamespace: kr.CanSetNamespace,
		Namespace:    kr.Namespace,
		Name:         secrets.ServerSecretName(r.Focus.Server),
	})

	return nil
}

func fillData(ctx context.Context, server *schema.Server, env *schema.Environment, col *secrets.Collection, r provisioning.StackRequest) (map[string][]byte, error) {
	var count int
	for _, userManaged := range col.UserManaged {
		count += len(userManaged)
	}

	if count == 0 {
		return nil, nil
	}

	snapshotKeys := r.Snapshots[keys.SnapshotKeys]

	contentSnapshots := map[string]fs.FS{}
	for key, snapshot := range r.Snapshots {
		if key != keys.SnapshotKeys {
			contentSnapshots[key] = snapshot
		}
	}

	var bundles []*fnsecrets.Bundle
	var bundleNames []string

	for snapshotPath, snapshot := range contentSnapshots {
		if err := fnfs.VisitFiles(ctx, snapshot, func(path string, blob bytestream.ByteStream, de fs.DirEntry) error {
			if filepath.Ext(path) != ".secrets" {
				return nil
			}

			if snapshotKeys == nil {
				return fmt.Errorf("can't use encrypted secrets without keys")
			}

			contents, err := bytestream.ReadAll(blob)
			if err != nil {
				return err
			}

			bundle, err := fnsecrets.LoadBundle(ctx, snapshotKeys, contents)
			if err != nil {
				return fnerrors.New("%s: %w", snapshotPath, err)
			}

			bundles = append(bundles, bundle)
			if sliceContains(bundleNames, path) {
				return fnerrors.InternalError("%s: multiple secret bundles with the same name? saw: %s", snapshotPath, strings.Join(bundleNames, "; "))
			}

			bundleNames = append(bundleNames, path)
			return nil
		}); err != nil {
			return nil, err
		}
	}

	data := map[string][]byte{}
	for k, userManaged := range col.UserManaged {
		for j, secret := range userManaged {
			var foundValue []byte
			var foundIn []string

			key := &fnsecrets.ValueKey{PackageName: col.InstanceOwners[k], Key: secret.Name, EnvironmentName: env.Name}

			for idx, bundle := range bundles {
				value, err := bundle.Lookup(ctx, key)
				if err != nil {
					return nil, err
				}
				if value != nil {
					foundValue = value
					foundIn = append(foundIn, bundleNames[idx])
				}
			}

			switch len(foundIn) {
			case 0:
				if !secret.Optional {
					return nil, fnerrors.UsageError(
						fmt.Sprintf("Try running `ns secrets set %s --secret %s:%s`", server.PackageName, key.PackageName, key.Key),
						"secret %q required by %q not specified", key.Key, key.PackageName)
				} else {
					// XXX should not mutate DevMap here.
					for _, x := range col.DevMap.Configure {
						if x.PackageName == key.PackageName {
							for k, s := range x.Secret {
								if s.Name == secret.Name {
									x.Secret[k] = nil
								}
							}
						}
					}
				}
			case 1:
				data[col.Names[k][j]] = foundValue
			default:
				return nil, fnerrors.NewWithLocation(server, "%s: secret %s:%s found in multiple files: %s",
					server.PackageName, key.PackageName, key.Key, strings.Join(foundIn, "; "))
			}
		}
	}

	return data, nil
}

func sliceContains(strs []string, str string) bool {
	for _, s := range strs {
		if s == str {
			return true
		}
	}
	return false
}
