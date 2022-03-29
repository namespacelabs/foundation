// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	corev1 "k8s.io/api/core/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/internal/keys"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/provision/tool/bootstrap"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/secrets"
)

type tool struct{}

func main() {
	bootstrap.RunTool(tool{})
}

func (tool) Apply(ctx context.Context, r bootstrap.Request, out *bootstrap.ApplyOutput) error {
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

	devMap, data, err := secrets.CollectSecrets(ctx, r.Focus.Server, contents)
	if err != nil {
		return err
	}

	// XXX LoadDevMap() assumes textproto; eventually should change this to binary.
	devMapBytes, err := prototext.Marshal(devMap)
	if err != nil {
		return err
	}

	devMapJSON, err := protojson.Marshal(devMap)
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

	name := secretName(r.Focus.Server)

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

	volId := fmt.Sprintf("%s-secrets-namespacelabs-dev", r.Focus.Server.Id)

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

	return nil
}

func (tool) Delete(ctx context.Context, r bootstrap.Request, out *bootstrap.DeleteOutput) error {
	namespace := kubetool.FromRequest(r).Namespace

	out.Ops = append(out.Ops, kubedef.Delete{
		Description: "server secrets",
		Resource:    "secrets",
		Namespace:   namespace,
		Name:        secretName(r.Focus.Server),
	})

	return nil
}

func secretName(srv *schema.Server) string {
	return fmt.Sprintf("%s.secrets.namespacelabs.dev", srv.Id)
}