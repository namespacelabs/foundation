// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"context"
	"strings"

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

	// TODO revisit tag generation.
	// Registry protocol requires a tag. go-containerregistry uses "latest" by default.
	// ECR treats all tags as immutable. This does not combine well, so we infer a stable tag here.
	defaultTag := strings.TrimPrefix(tag.Digest, "sha256:")
	opts = append(opts, name.WithDefaultTag(defaultTag))

	return name.NewTag(tag.ImageRef(), opts...)
}
