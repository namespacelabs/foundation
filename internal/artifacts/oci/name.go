// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"namespacelabs.dev/foundation/schema"
)

type Keychain interface {
	Resolve(context.Context, authn.Resource) (authn.Authenticator, error)
}

type AllocatedName struct {
	InsecureRegistry bool
	Keychain         Keychain
	ImageID
}

func (t AllocatedName) ComputeDigest(context.Context) (schema.Digest, error) {
	return schema.DigestOf("insecureRegistry", t.InsecureRegistry, "repository", t.Repository, "tag", t.Tag, "digest", t.Digest)
}

func ParseTag(tag AllocatedName) (name.Tag, error) {
	var opts []name.Option
	if tag.InsecureRegistry {
		opts = append(opts, name.Insecure)
	}

	return name.NewTag(tag.ImageRef(), opts...)
}
