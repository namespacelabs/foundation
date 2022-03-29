// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package secrets

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/fs"
	"io/ioutil"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/schema"
)

const MountPath = "/secrets"

func CollectSecrets(ctx context.Context, server *schema.Server, contents fs.FS) (*SecretDevMap, map[string][]byte, error) {
	devMap := &SecretDevMap{}
	data := map[string][]byte{}

	for _, alloc := range server.Allocation {
		for _, instance := range alloc.Instance {
			merged := &Secrets{}
			for _, instantiate := range instance.GetInstantiated() {
				// XXX old path, remove soon.
				if instantiate.GetConstructor().GetTypeUrl() == "namespacelabs.dev/foundation/std/secrets/ns.Secret" {
					secret := &Secret{}
					if err := proto.Unmarshal(instantiate.GetConstructor().Value, secret); err != nil {
						return nil, nil, err
					}
					merged.Secret = append(merged.Secret, secret)
				}
				if instantiate.GetPackageName() == "namespacelabs.dev/foundation/std/secrets" && instantiate.GetType() == "Secret" {
					secret := &Secret{}
					if err := proto.Unmarshal(instantiate.GetConstructor().Value, secret); err != nil {
						return nil, nil, err
					}
					merged.Secret = append(merged.Secret, secret)
				}
			}

			if len(merged.Secret) == 0 {
				continue
			}

			var m map[string]*Value
			var err error
			if contents != nil {
				m, err = ProvideSecretsFromFS(ctx, contents, instance.InstanceOwner, merged)
				if err != nil {
					return nil, nil, err
				}
			}
			// XXX this is not quite right as it doesn't take into account the
			// allocation path.
			configure := &SecretDevMap_Configure{
				PackageName: instance.InstanceOwner,
			}

			for _, sec := range merged.Secret {
				// For development, each secret is maintained under a single
				// top-level server-scoped secret. We may want to revisit this
				// and break down the granularity of the secretblob. But
				// unfortunately Kubernete's resource name limitations make it
				// a bit unfortunate to maintain any kind of decent top-level
				// names.
				rawPath := fmt.Sprintf("%s/%s", instance.InstanceOwner, instance.AllocName)
				name := base64.RawStdEncoding.EncodeToString([]byte(rawPath)) + "." + sec.Name

				if contents != nil {
					if contains(sec.Provision, Provision_PROVISION_AS_FILE) {
						b, err := fs.ReadFile(contents, m[sec.Name].Path)
						if err != nil {
							return nil, nil, err
						}

						data[name] = b
					} else {
						data[name] = []byte(m[sec.Name].Value)
					}
				}

				// Tell the runtime where to find the secret data.
				configure.Secret = append(configure.Secret, &SecretDevMap_SecretSpec{
					Name:     sec.Name,
					FromPath: filepath.Join(MountPath, name),
				})
			}
			devMap.Configure = append(devMap.Configure, configure)
		}
	}

	return devMap, data, nil

}

func ProvideSecretsFromFS(ctx context.Context, src fs.FS, caller string, req *Secrets) (map[string]*Value, error) {
	sdm, err := loadDevMap(src)
	if err != nil {
		return nil, fmt.Errorf("%v: failed to provision secrets: %w", caller, err)
	}

	cfg := lookupConfig(sdm, caller)
	if cfg == nil {
		return nil, fmt.Errorf("%v: no secret configuration definition in map.textpb", caller)
	}

	result := map[string]*Value{}
	for _, s := range req.Secret {
		spec := lookupSecret(cfg, s.Name)
		if spec == nil {
			return nil, fmt.Errorf("no secret configuration for %s of %q in map.textpb", s.Name, caller)
		}

		value := &Value{
			Name:      s.Name,
			Provision: s.Provision,
		}

		result[s.Name] = value

		if contains(s.Provision, Provision_PROVISION_AS_FILE) {
			if spec.FromPath == "" {
				return nil, fmt.Errorf("requested secret %s by file, but the provider set it by value; and we don't implement secure secret writing yet", s.Name)
			}
			value.Path = spec.FromPath
		}

		if contains(s.Provision, Provision_PROVISION_INLINE) {
			if spec.FromPath != "" {
				var contents []byte
				var err error

				if filepath.IsAbs(spec.FromPath) {
					contents, err = ioutil.ReadFile(spec.FromPath)
				} else {
					contents, err = fs.ReadFile(src, spec.FromPath)
				}

				if err != nil {
					return nil, fmt.Errorf("failed while reading secret %s: %w", s.Name, err)
				}
				value.Value = []byte(strings.TrimSpace(string(contents)))
			} else {
				value.Value = []byte(spec.Value)
			}
		}
	}

	return result, nil
}

func contains(provisions []Provision, provision Provision) bool {
	for _, p := range provisions {
		if p == provision {
			return true
		}
	}
	return false
}

func loadDevMap(src fs.FS) (*SecretDevMap, error) {
	mapContents, err := fs.ReadFile(src, "map.textpb")
	if err != nil {
		return nil, err
	}

	sdm := &SecretDevMap{}
	if err := prototext.Unmarshal(mapContents, sdm); err != nil {
		return nil, err
	}

	return sdm, nil
}

func lookupConfig(sdm *SecretDevMap, caller string) *SecretDevMap_Configure {
	for _, c := range sdm.Configure {
		if c.PackageName == caller {
			return c
		}
	}

	return nil
}

func lookupSecret(c *SecretDevMap_Configure, name string) *SecretDevMap_SecretSpec {
	for _, s := range c.Secret {
		if s.Name == name {
			return s
		}
	}

	return nil
}