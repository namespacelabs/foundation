// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// available at http://github.com/namespacelabs/foundation

package download

import (
	"context"
	"io/ioutil"

	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

// Returns a `Computable` downloading an artifact. Verifies against a known `schema.Digest`.
func URL(ref artifacts.Reference) compute.Computable[bytestream.ByteStream] {
	return &downloadUrl{
		url:      ref.URL,
		digest:   compute.Immediate(ref.Digest),
		artifact: FetchUrl(ref.URL)}
}

// Returns a `Computable` downloading both an artifact and its expected checksum.
// Verifies the artifact against the checksum.
func UrlAndDigest(artifactUrl, digestUrl, digestAlg string) compute.Computable[bytestream.ByteStream] {
	downloadDigest := compute.Transform(
		FetchUrl(digestUrl),
		func(ctx context.Context, digest bytestream.ByteStream) (schema.Digest, error) {
			r, err := digest.Reader()
			if err != nil {
				return schema.Digest{}, err
			}
			v, err := ioutil.ReadAll(r)
			if err != nil {
				return schema.Digest{}, err
			}
			return schema.Digest{Algorithm: digestAlg, Hex: string(v)}, nil
		})
	return &downloadUrl{url: artifactUrl, digest: downloadDigest, artifact: FetchUrl(artifactUrl)}
}

type downloadUrl struct {
	url      string
	digest   compute.Computable[schema.Digest]
	artifact compute.Computable[bytestream.ByteStream]

	compute.LocalScoped[bytestream.ByteStream]
}

func (dl *downloadUrl) Action() *tasks.ActionEvent {
	return tasks.Action("artifact.verify").Arg("url", dl.url)
}

func (dl *downloadUrl) Inputs() *compute.In {
	return compute.Inputs().Computable("artifact", dl.artifact).Computable("digest", dl.digest)
}

func (dl *downloadUrl) Compute(ctx context.Context, deps compute.Resolved) (bytestream.ByteStream, error) {
	artifactBytes := compute.MustGetDepValue(deps, dl.artifact, "artifact")
	artifactDigest, err := bytestream.Digest(ctx, artifactBytes)
	if err != nil {
		return nil, err
	}

	expectedDigest := compute.MustGetDepValue(deps, dl.digest, "digest")
	if !artifactDigest.Equals(expectedDigest) {
		return nil, fnerrors.InternalError("artifact.verify: %s: digest didn't match, got %s expected %s", dl.url, artifactDigest, expectedDigest)
	}

	// XXX support returning a io.Reader here so we don't need to buffer the download.
	return artifactBytes, nil
}
