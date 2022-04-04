// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package secrets

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"io/fs"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

var (
	ScopedMountPath = "/secrets"
	MountPath       = filepath.Join(ScopedMountPath, "server")
)

type Collection2 struct {
	DevMap *SecretDevMap
	Data   map[string][]byte // User managed data; only present if contents were provided.
}

type Collection struct {
	DevMap         *SecretDevMap
	UserManaged    [][]*Secret // Indexing is the same as `DevMap.Configure`.
	InstanceOwners []string    // Indexing is the same as `DevMap.Configure`.
	Names          [][]string  // Indexing is the same as `DevMap.Configure`.
	Generated      []Generated
}

type Generated struct {
	UniqueID string
	BasePath string
	Secrets  []*Secret
}

var (
	validIdRe           = regexp.MustCompile("^[a-z][0123456789abcdefghijklmnopqrstuvwyxz]{7,15}$")
	lowerCaseBase32Raw  = "0123456789abcdefghijklmnopqrstuv"
	base32enc           = base32.NewEncoding(lowerCaseBase32Raw).WithPadding(base32.NoPadding)
	reservedSecretNames = []string{"server"}
)

func Collect(server *schema.Server) (*Collection, error) {
	devMap := &SecretDevMap{}
	col := &Collection{DevMap: devMap}

	for _, alloc := range server.Allocation {
		for _, instance := range alloc.Instance {
			var generated, userManaged []*Secret
			for _, instantiate := range instance.GetInstantiated() {
				if instantiate.GetPackageName() == "namespacelabs.dev/foundation/std/secrets" && instantiate.GetType() == "Secret" {
					secret := &Secret{}
					if err := proto.Unmarshal(instantiate.GetConstructor().Value, secret); err != nil {
						return nil, err
					}
					if secret.Generate != nil {
						if secret.Generate.UniqueId == "" {
							h := sha256.New()
							fmt.Fprint(h, instance.InstanceOwner)
							secret.Generate.UniqueId = base32enc.EncodeToString(h.Sum(nil)[:8])
						} else if slices.Contains(reservedSecretNames, secret.Generate.UniqueId) {
							return nil, fnerrors.UserError(nil, "bad unique secret id: %q (is a reserved word)", secret.Generate.UniqueId)
						} else if !validIdRe.MatchString(secret.Generate.UniqueId) {
							return nil, fnerrors.UserError(nil, "bad unique secret id: %q (must be alphanumeric, between 8 and 16 characters)", secret.Generate.UniqueId)
						}
						generated = append(generated, secret)
					} else {
						userManaged = append(userManaged, secret)
					}
				}
			}

			// XXX this is not quite right as it doesn't take into account the
			// allocation path.
			configure := &SecretDevMap_Configure{
				PackageName: instance.InstanceOwner,
			}

			var names []string
			if len(userManaged) > 0 {
				for _, sec := range userManaged {
					// For development, each secret is maintained under a single
					// top-level server-scoped secret. We may want to revisit this
					// and break down the granularity of the secretblob. But
					// unfortunately Kubernete's resource name limitations make it
					// a bit unfortunate to maintain any kind of decent top-level
					// names.
					rawPath := fmt.Sprintf("%s/%s", instance.InstanceOwner, instance.AllocName)
					name := base64.RawStdEncoding.EncodeToString([]byte(rawPath)) + "." + sec.Name
					names = append(names, name)

					// Tell the runtime where to find the secret data.
					configure.Secret = append(configure.Secret, &SecretDevMap_SecretSpec{
						Name:     sec.Name,
						FromPath: filepath.Join(MountPath, name),
					})
				}
			}

			if len(generated) > 0 {
				byUniqueID := map[string][]*Secret{}
				for _, sec := range generated {
					configure.Secret = append(configure.Secret, &SecretDevMap_SecretSpec{
						Name:     sec.Name,
						FromPath: filepath.Join(ScopedMountPath, strings.ReplaceAll(instance.InstanceOwner, "/", "-")+"-"+sec.Generate.UniqueId, sec.Name),
					})
					byUniqueID[sec.Generate.UniqueId] = append(byUniqueID[sec.Generate.UniqueId], sec)
				}

				for id, group := range byUniqueID {
					seen := map[string]*Secret{}
					for _, secret := range group {
						if existing := seen[secret.Name]; existing != nil {
							if !proto.Equal(existing, secret) {
								return nil, fnerrors.UserError(nil, "%s: %s: incompatible secret definition", id, secret.Name)
							}
						} else {
							seen[secret.Name] = secret
						}
					}

					gen := Generated{
						UniqueID: id,
						BasePath: filepath.Join(ScopedMountPath, strings.ReplaceAll(instance.InstanceOwner, "/", "-")+"-"+id),
					}
					for _, secret := range seen {
						gen.Secrets = append(gen.Secrets, secret)
					}

					slices.SortFunc(gen.Secrets, func(a, b *Secret) bool {
						return strings.Compare(a.Name, b.Name) < 0
					})

					col.Generated = append(col.Generated, gen)
				}
			}

			if len(configure.Secret) > 0 {
				devMap.Configure = append(devMap.Configure, configure)
				col.InstanceOwners = append(col.InstanceOwners, instance.InstanceOwner)
				col.Names = append(col.Names, names)
				col.UserManaged = append(col.UserManaged, userManaged)
			}
		}
	}

	slices.SortFunc(col.Generated, func(a, b Generated) bool {
		return strings.Compare(a.UniqueID, b.UniqueID) < 0
	})

	return col, nil
}

func FillData(ctx context.Context, col *Collection, contents fs.FS) (map[string][]byte, error) {
	data := map[string][]byte{}
	for k, userManaged := range col.UserManaged {
		m, err := ProvideSecretsFromFS(ctx, contents, col.InstanceOwners[k], userManaged...)
		if err != nil {
			return nil, err
		}

		names := col.Names[k]
		for j, sec := range userManaged {
			name := names[j]
			if contains(sec.Provision, Provision_PROVISION_AS_FILE) {
				b, err := fs.ReadFile(contents, m[sec.Name].Path)
				if err != nil {
					return nil, err
				}

				data[name] = b
			} else {
				data[name] = []byte(m[sec.Name].Value)
			}
		}
	}

	return data, nil
}

func (col *Collection) SecretsOf(packageName string) []*SecretDevMap_SecretSpec {
	for _, conf := range col.DevMap.Configure {
		if conf.PackageName == packageName {
			return conf.Secret
		}
	}

	return nil
}

func ProvideSecretsFromFS(ctx context.Context, src fs.FS, caller string, userManaged ...*Secret) (map[string]*Value, error) {
	sdm, err := loadDevMap(src)
	if err != nil {
		return nil, fmt.Errorf("%v: failed to provision secrets: %w", caller, err)
	}

	cfg := lookupConfig(sdm, caller)
	if cfg == nil {
		return nil, fmt.Errorf("%v: no secret configuration definition in map.textpb", caller)
	}

	result := map[string]*Value{}
	for _, s := range userManaged {
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
