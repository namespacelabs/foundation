// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/keys"
	fnsecrets "namespacelabs.dev/foundation/internal/secrets"
	"namespacelabs.dev/foundation/provision/configure"
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

	collection, err := secrets.Collect(r.Focus.Server)
	if err != nil {
		return err
	}

	data, err := fillData(ctx, r.Focus.Server, r.Env, collection, r)
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

	if data == nil {
		data = map[string][]byte{}
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
			WithType(v1.SecretTypeOpaque).
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
				_, _ = rand.Reader.Read(raw)
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

func fillData(ctx context.Context, server *schema.Server, env *schema.Environment, col *secrets.Collection, r configure.StackRequest) (map[string][]byte, error) {
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

	// Legacy path.
	if secrets, ok := contentSnapshots["secrets"]; ok {
		if len(contentSnapshots) > 1 {
			return nil, fnerrors.UserError(nil, "use of old-style secrets/ directory and secret bundles are mutually exclusive")
		}

		contents, err := loadSnapshot(ctx, secrets, snapshotKeys)
		if err != nil {
			return nil, err
		}

		data := map[string][]byte{}
		for k, userManaged := range col.UserManaged {
			if len(userManaged) == 0 {
				continue
			}

			m, err := provideSecretsFromFS(ctx, contents, col.InstanceOwners[k], userManaged...)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", server.PackageName, err)
			}

			names := col.Names[k]
			for j, sec := range userManaged {
				name := names[j]
				data[name] = m[sec.Name]
			}
		}

		return data, nil
	}

	var bundles []*fnsecrets.Bundle
	var bundleNames []string

	for _, snapshot := range contentSnapshots {
		if err := fnfs.VisitFiles(ctx, snapshot, func(path string, contents []byte, de fs.DirEntry) error {
			if filepath.Ext(path) != ".secrets" {
				return nil
			}

			if snapshotKeys == nil {
				return fmt.Errorf("can't use encrypted secrets without keys")
			}

			bundle, err := fnsecrets.LoadBundle(ctx, snapshotKeys, contents)
			if err != nil {
				return err
			}

			bundles = append(bundles, bundle)
			if sliceContains(bundleNames, path) {
				return fnerrors.InternalError("multiple secret bundles with the same name? saw: %s", strings.Join(bundleNames, "; "))
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
				return nil, fnerrors.UsageError(
					fmt.Sprintf("Try running `fn secrets set %s --secret %s:%s`", server.PackageName, key.PackageName, key.Key),
					"secret %q required by %q not specified", key.Key, key.PackageName)
			case 1:
				data[col.Names[k][j]] = foundValue
			default:
				return nil, fnerrors.UserError(server, "%s: secret %s:%s found in multiple files: %s",
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

func loadSnapshot(ctx context.Context, contents, keyDir fs.FS) (fs.FS, error) {
	archive, err := contents.Open(keys.EncryptedFile)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	} else if err == nil {
		defer archive.Close()

		if keyDir == nil {
			return nil, fmt.Errorf("can't use encrypted secrets without keys")
		}

		contents, err = keys.DecryptAsFS(ctx, keyDir, archive)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt: %w", err)
		}
	}

	return contents, nil
}

func provideSecretsFromFS(ctx context.Context, src fs.FS, caller string, userManaged ...*secrets.Secret) (map[string][]byte, error) {
	sdm, err := secrets.LoadDevMap(src)
	if err != nil {
		return nil, fmt.Errorf("%v: failed to provision secrets: %w", caller, err)
	}

	cfg := secrets.LookupConfig(sdm, caller)
	if cfg == nil {
		return nil, fmt.Errorf("no secret configuration for %q", caller)
	}

	result := map[string][]byte{}
	for _, s := range userManaged {
		spec := lookupSecret(cfg, s.Name)
		if spec == nil {
			return nil, fmt.Errorf("no secret configuration for %s of %q", s.Name, caller)
		}

		if spec.FromPath != "" {
			var contents []byte
			var err error

			if filepath.IsAbs(spec.FromPath) {
				return nil, fmt.Errorf("%s: %s: absolute paths are not supported in devmaps", caller, s.Name)
			}

			contents, err = fs.ReadFile(src, spec.FromPath)
			if err != nil {
				return nil, fmt.Errorf("failed while reading secret %s: %w", s.Name, err)
			}
			result[s.Name] = []byte(strings.TrimSpace(string(contents)))
		} else {
			result[s.Name] = []byte(spec.Value)
		}
	}

	return result, nil
}

func lookupSecret(c *secrets.SecretDevMap_Configure, name string) *secrets.SecretDevMap_SecretSpec {
	for _, s := range c.Secret {
		if s.Name == name {
			return s
		}
	}

	return nil
}
