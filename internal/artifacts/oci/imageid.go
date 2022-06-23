// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"namespacelabs.dev/foundation/schema"
)

type ImageID struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Digest     string `json:"digest"`
}

func (i ImageID) ImageRef() string {
	r := i.Repository
	if i.Tag != "" {
		r += ":" + i.Tag
	}
	if i.Digest != "" {
		r += "@" + i.Digest
	}
	return r
}

func (i ImageID) String() string {
	return i.ImageRef()
}

func (i ImageID) RepoAndDigest() string {
	if i.Digest != "" {
		// XXX security: consider enforcing the use of digests
		return i.Repository + "@" + i.Digest
	}
	return i.ImageRef()
}
func (i ImageID) WithDigest(d fmt.Stringer) ImageID {
	return ImageID{Repository: i.Repository, Tag: i.Tag, Digest: d.String()}
}

func (i ImageID) ComputeDigest(ctx context.Context) (schema.Digest, error) {
	return schema.DigestOf("repository", i.Repository, "tag", i.Tag, "digest", i.Digest)
}

func ParseImageID(ref string) (ImageID, error) {
	parts := strings.SplitN(ref, "@", 2)

	t, err := name.NewTag(parts[0], name.WithDefaultTag(""))
	if err != nil {
		return ImageID{}, err
	}

	i := ImageID{Repository: t.Repository.Name(), Tag: t.TagStr()}

	if len(parts) == 2 {
		i.Digest = parts[1]
	}

	return i, nil
}
