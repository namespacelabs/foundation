// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package download

import (
	"bytes"
	"context"
	"io"

	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

// Returns a `Computable` downloading an artifact. Verifies against a known `schema.Digest`.
// TODO inject computables (to make this code testable).
func URL(ref artifacts.Reference) compute.Computable[bytestream.ByteStream] {
	return &urlAndDigest{
		url:      ref.URL,
		digest:   compute.Immediate(ref.Digest),
		artifact: DownloadUrl(ref.URL)}
}

// Returns a `Computable` downloading both an artifact and its expected checksum.
// Verifies the artifact against the checksum.
// TODO inject computables (to make this code testable).
func UrlAndDigest(artifactUrl, digestUrl, digestAlg string) compute.Computable[bytestream.ByteStream] {
	downloadDigest := compute.Transform(
		DownloadUrl(digestUrl),
		func(ctx context.Context, digest bytestream.ByteStream) (schema.Digest, error) {
			r, err := digest.Reader()
			if err != nil {
				return schema.Digest{}, err
			}
			var buf bytes.Buffer
			if _, err = io.Copy(&buf, r); err != nil {
				return schema.Digest{}, err
			}
			return schema.Digest{Algorithm: digestAlg, Hex: buf.String()}, nil
		})
	return &urlAndDigest{url: artifactUrl, digest: downloadDigest, artifact: DownloadUrl(artifactUrl)}
}

type urlAndDigest struct {
	url      string // For presentation only.
	digest   compute.Computable[schema.Digest]
	artifact compute.Computable[bytestream.ByteStream]

	compute.LocalScoped[bytestream.ByteStream]
}

func (dl *urlAndDigest) Action() *tasks.ActionEvent {
	return tasks.Action("artifact.verify").Arg("url", dl.url)
}

func (dl *urlAndDigest) Inputs() *compute.In {
	return compute.Inputs().Computable("artifact", dl.artifact).Computable("digest", dl.digest)
}

func (dl *urlAndDigest) Compute(ctx context.Context, deps compute.Resolved) (bytestream.ByteStream, error) {
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
