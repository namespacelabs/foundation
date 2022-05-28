// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
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

func URL(ref artifacts.Reference) compute.Computable[bytestream.ByteStream] {
	instantDigest := compute.Map(
		tasks.Action("artifact.computeddigest"),
		compute.Inputs(),
		compute.Output{},
		func(ctx context.Context, _ compute.Resolved) (schema.Digest, error) {
			return ref.Digest, nil
		})
	return &downloadUrl{url: ref.URL, expected: instantDigest, fetch: FetchUrl(ref.URL)}
}

func Url(url string, digest compute.Computable[schema.Digest]) compute.Computable[bytestream.ByteStream] {
	return &downloadUrl{url: url, expected: digest, fetch: FetchUrl(url)}
}

func fetchDigest(algorithm, digestUrl string) compute.Computable[schema.Digest] {
	fetchDigest := FetchUrl(digestUrl)
	digestC := compute.Map(
		tasks.Action("artifact.fetch").Arg("algorithm", algorithm).Arg("digesturl", digestUrl),
		compute.Inputs().Computable("digest", fetchDigest),
		compute.Output{},
		func(ctx context.Context, deps compute.Resolved) (schema.Digest, error) {
			digest, _ := compute.GetDep(deps, fetchDigest, "digest")
			reader, err := digest.Value.Reader()
			if err != nil {
				return schema.Digest{}, err
			}
			v, err := ioutil.ReadAll(reader)
			if err != nil {
				return schema.Digest{}, err
			}
			return schema.Digest{Algorithm: algorithm, Hex: string(v)}, nil
		})
	return digestC
}

func Url22(url, digestUrl, algorithm string) compute.Computable[bytestream.ByteStream] {
	return &downloadUrl{url: url, expected: fetchDigest(algorithm, digestUrl), fetch: FetchUrl(url)}
}

// Computable fetching and verifying a URL.
type downloadUrl struct {
	url      string
	expected compute.Computable[schema.Digest]
	fetch    compute.Computable[bytestream.ByteStream]

	compute.LocalScoped[bytestream.ByteStream]
}

func (dl *downloadUrl) Action() *tasks.ActionEvent {
	return tasks.Action("artifact.verify").Arg("url", dl.url).Arg("digest", dl.expected)
}

func (dl *downloadUrl) Inputs() *compute.In {
	return compute.Inputs().Computable("url", dl.fetch).Computable("digest", dl.expected)
}

func (dl *downloadUrl) Compute(ctx context.Context, deps compute.Resolved) (bytestream.ByteStream, error) {
	bs, _ := compute.GetDep(deps, dl.fetch, "url")

	resultDigest, err := bytestream.Digest(ctx, bs.Value)
	if err != nil {
		return nil, err
	}

	digest, _ := compute.GetDep(deps, dl.expected, "digest")
	if !resultDigest.Equals(digest.Value) {
		return nil, fnerrors.InternalError("artifact.verify: %s: digest didn't match, got %s expected %s", dl.url, resultDigest, digest.Value)
	}

	// XXX support returning a io.Reader here so we don't need to buffer the download.
	return bs.Value, nil
}
