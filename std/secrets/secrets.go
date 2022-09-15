// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package secrets

import (
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/support/naming"
	"namespacelabs.dev/foundation/schema"
)

var (
	ScopedMountPath = "/secrets"
	MountPath       = filepath.Join(ScopedMountPath, "server")
)

type Collection struct {
	DevMap         *SecretDevMap
	UserManaged    [][]*Secret // Indexing is the same as `DevMap.Configure`.
	InstanceOwners []string    // Indexing is the same as `DevMap.Configure`.
	Names          [][]string  // Indexing is the same as `DevMap.Configure`.
	Generated      []Generated
}

type Generated struct {
	ID           string
	Path         string
	ResourceName string
	Secret       *Secret
}

var (
	validIdRe           = regexp.MustCompile("^[a-z][0123456789abcdefghijklmnopqrstuvwyxz]{7,15}$")
	validNameRe         = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9-_]{0,31}$")
	lowerCaseBase32Raw  = "0123456789abcdefghijklmnopqrstuv"
	base32enc           = base32.NewEncoding(lowerCaseBase32Raw).WithPadding(base32.NoPadding)
	reservedSecretNames = map[string]struct{}{"server": {}}
)

func Collect(server *schema.Server) (*Collection, error) {
	devMap := &SecretDevMap{}
	col := &Collection{DevMap: devMap}

	for _, alloc := range server.Allocation {
		for _, instance := range alloc.Instance {
			var generated, userManaged []*Secret
			// TODO handle scoped instantiations
			for _, instantiate := range instance.GetInstantiated() {
				if instantiate.GetPackageName() == "namespacelabs.dev/foundation/std/secrets" && instantiate.GetType() == "Secret" {
					secret := &Secret{}
					if err := proto.Unmarshal(instantiate.GetConstructor().Value, secret); err != nil {
						return nil, err
					}

					if !validNameRe.MatchString(secret.Name) {
						return nil, fnerrors.UserError(nil, "bad secret name: %q (must be alphanumeric, up to 32 characters)", secret.Name)
					}

					if secret.InitializeWith != nil || secret.SelfSignedTlsCertificate != nil {
						if secret.Generate == nil {
							secret.Generate = &GenerateSpecification{}
						}
					}

					if secret.Generate != nil {
						if secret.Generate.UniqueId == "" {
							secret.Generate.UniqueId = naming.StableIDN(instance.InstanceOwner, 16)
						} else if _, ok := reservedSecretNames[secret.Generate.UniqueId]; ok {
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
						Name:         sec.Name,
						FromPath:     filepath.Join(MountPath, name),
						ResourceName: ServerSecretName(server),
					})
				}
			}

			if len(generated) > 0 {
				byUniqueID := map[string][]*Secret{}
				for _, sec := range generated {
					byUniqueID[sec.Generate.UniqueId] = append(byUniqueID[sec.Generate.UniqueId], sec)
					id := strings.Join([]string{sec.Name, sec.Generate.UniqueId}, "-")
					path := filepath.Join(ScopedMountPath, strings.ReplaceAll(instance.InstanceOwner, "/", "-"), id)

					// XXX This leaks Kubernetes.
					resourceName := id + ".managed.namespacelabs.dev"

					configure.Secret = append(configure.Secret, &SecretDevMap_SecretSpec{
						Name: sec.Name,
						// By convention, the generated k8s secret has a single secret inside, with
						// the actual secret name.
						FromPath:     filepath.Join(path, sec.Name),
						ResourceName: resourceName,
					})

					col.Generated = append(col.Generated, Generated{
						ID:           id,
						ResourceName: resourceName,
						Path:         path,
						Secret:       sec,
					})
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

	sort.Slice(col.Generated, func(i, j int) bool {
		return strings.Compare(col.Generated[i].ID, col.Generated[j].ID) < 0
	})

	return col, nil
}

func (col *Collection) SecretsOf(packageName string) []*SecretDevMap_SecretSpec {
	for _, conf := range col.DevMap.Configure {
		if conf.PackageName == packageName {
			return conf.Secret
		}
	}

	return nil
}

func LoadSourceDevMap(src fs.FS) (*SecretDevMap, error) {
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

func LoadBinaryDevMap(src fs.FS) (*SecretDevMap, error) {
	mapContents, err := fs.ReadFile(src, "map.binarypb")
	if err != nil {
		return nil, err
	}

	sdm := &SecretDevMap{}
	if err := proto.Unmarshal(mapContents, sdm); err != nil {
		return nil, err
	}

	return sdm, nil
}

func LookupConfig(sdm *SecretDevMap, caller string) *SecretDevMap_Configure {
	for _, c := range sdm.Configure {
		if c.PackageName == caller {
			return c
		}
	}

	return nil
}

func ServerSecretName(srv *schema.Server) string {
	return strings.Join([]string{srv.Name, srv.Id}, "-") + ".managed.namespacelabs.dev"
}
