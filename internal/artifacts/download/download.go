// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package download

import (
	"context"
	"io"
	"net/http"

	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/ctxio"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fntypes"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func URL(ref artifacts.Reference) compute.Computable[compute.ByteStream] {
	return &downloadUrl{url: ref.URL, digest: ref.Digest}
}

type downloadUrl struct {
	url    string
	digest fntypes.Digest

	compute.LocalScoped[compute.ByteStream]
}

func (dl *downloadUrl) Action() *tasks.ActionEvent {
	return tasks.Action("artifact.download").Arg("url", dl.url).Arg("digest", dl.digest)
}

func (dl *downloadUrl) Inputs() *compute.In {
	return compute.Inputs().Str("url", dl.url).Digest("digest", dl.digest)
}

func (dl *downloadUrl) Compute(ctx context.Context, _ compute.Resolved) (compute.ByteStream, error) {
	resp, err := http.Get(dl.url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	bsw, err := compute.NewByteStream(ctx)
	if err != nil {
		return nil, err
	}

	defer bsw.Close()

	var p artifacts.ProgressWriter
	if resp.ContentLength >= 0 {
		p = artifacts.NewProgressWriter(uint64(resp.ContentLength), nil)
	} else {
		p = artifacts.NewProgressWriter(0, nil)
	}

	tasks.Attachments(ctx).SetProgress(p)

	w := io.MultiWriter(bsw, p)

	_, err = io.Copy(ctxio.WriterWithContext(ctx, w, nil), resp.Body)
	if err != nil {
		return nil, err
	}

	bs, err := bsw.Complete()
	if err != nil {
		return nil, err
	}

	if resultDigest := bs.Digest(); resultDigest != dl.digest {
		return nil, fnerrors.InternalError("artifact.download: %s: digest didn't match, got %s expected %s", dl.url, resultDigest, dl.digest)
	}

	// XXX support returning a io.Reader here so we don't need to buffer the download.
	return bs, nil
}