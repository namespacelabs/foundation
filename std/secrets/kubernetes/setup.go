// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/internal/keys"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/go-ids"
)

type tool struct{}

func main() {
	configure.RunTool(tool{})
}

func (tool) Apply(ctx context.Context, r configure.StackRequest, out *configure.ApplyOutput) error {
	namespace := kubetool.FromRequest(r).Namespace

	contents := r.Snapshots["secrets"]
	if contents == nil {
		return fmt.Errorf("secrets snapshot is missing from input")
	}

	archive, err := contents.Open(keys.EncryptedFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	} else if err == nil {
		defer archive.Close()

		keyDir := r.Snapshots[deploy.SnapshotKeys]
		if keyDir == nil {
			return fmt.Errorf("can't use encrypted secrets without keys")
		}

		contents, err = keys.DecryptAsFS(ctx, keyDir, archive)
		if err != nil {
			return fmt.Errorf("failed to decrypt: %w", err)
		}
	}

	collection, err := secrets.Collect(r.Focus.Server)
	if err != nil {
		return err
	}

	data, err := secrets.FillData(ctx, collection, contents)
	if err != nil {
		return err
	}

	// XXX LoadDevMap() assumes textproto; eventually should change this to binary.
	devMapBytes, err := prototext.Marshal(collection.DevMap)
	if err != nil {
		return err
	}

	devMapJSON, err := protojson.Marshal(collection.DevMap)
	if err != nil {
		return err
	}

	// We do something funky here, we keep the mapping as a secret, so that
	// k8s also maps it to the container's namespace, and we can load it
	// using our regular mechanisms. Within that dev map, we map each
	// secret to other files mounted as well.
	data["map.textpb"] = devMapBytes

	// We also include a JSON version of the map to facilitiate JS-based uses.
	data["map.json"] = devMapJSON

	name := serverSecretName(r.Focus.Server)

	out.Definitions = append(out.Definitions, kubedef.Apply{
		Description: "server secrets",
		Resource:    "secrets",
		Namespace:   namespace,
		Name:        name,
		Body: applycorev1.
			Secret(name, namespace).
			WithType(corev1.SecretTypeOpaque).
			WithAnnotations(kubedef.MakeAnnotations(r.Stack.GetServer(r.Focus.GetPackageName()))).
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
						SecretName: name,
					},
				},
			}},
		}})

	out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
		With: &kubedef.ContainerExtension{
			VolumeMount: []*kubedef.ContainerExtension_VolumeMount{{
				Name:        volId,
				ReadOnly:    true,
				MountPath:   secrets.MountPath,
				MountOnInit: true, // Allow secret access during server initialization
			}},
		}})

	for _, gen := range collection.Generated {
		name := gen.ID + ".managed.namespacelabs.dev"
		volId := "fn-secret-" + gen.ID

		out.Extensions = append(out.Extensions, kubedef.ExtendSpec{
			With: &kubedef.SpecExtension{
				Volume: []*kubedef.SpecExtension_Volume{{
					Name: volId,
					VolumeType: &kubedef.SpecExtension_Volume_Secret_{
						Secret: &kubedef.SpecExtension_Volume_Secret{
							SecretName: name,
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

		if gen.Secret.InitializeWith == nil {
			data := map[string][]byte{}
			switch gen.Secret.Generate.Format {
			case secrets.GenerateSpecification_FORMAT_BASE32:
				data[gen.Secret.Name] = []byte(ids.NewRandomBase32ID(int(gen.Secret.Generate.RandomByteCount)))
			default: // Including BASE64
				raw := make([]byte, gen.Secret.Generate.RandomByteCount)
				rand.Reader.Read(raw)
				data[gen.Secret.Name] = []byte(base64.RawStdEncoding.EncodeToString(raw))
			}

			newSecret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Labels:    kubedef.MakeLabels(r.Env, nil),
				},
				Data: data,
			}

			out.Definitions = append(out.Definitions, kubedef.Create{
				Description: "Generated server secrets",
				IfMissing:   true,
				Resource:    "secrets",
				Namespace:   namespace,
				Name:        name,
				Body:        newSecret,
			})
		} else {
			out.Definitions = append(out.Definitions, kubedef.CreateSecretConditionally{
				Description:       "Generated server secrets",
				Namespace:         namespace,
				Name:              name,
				UserSpecifiedName: gen.Secret.Name,
				Invocation:        gen.Secret.InitializeWith,
			})
		}

	}

	return nil
}

func (tool) Delete(ctx context.Context, r configure.StackRequest, out *configure.DeleteOutput) error {
	namespace := kubetool.FromRequest(r).Namespace

	out.Ops = append(out.Ops, kubedef.Delete{
		Description: "server secrets",
		Resource:    "secrets",
		Namespace:   namespace,
		Name:        serverSecretName(r.Focus.Server),
	})

	return nil
}

func serverSecretName(srv *schema.Server) string {
	return strings.Join([]string{srv.Name, srv.Id}, "-") + ".managed.namespacelabs.dev"
}
