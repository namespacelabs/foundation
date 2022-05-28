// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package download

import (
	"context"
	"io"
	"net/http"

	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/ctxio"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func FetchUrl(url string) compute.Computable[bytestream.ByteStream] {
	return &fetch{url: url}
}

type fetch struct {
	url string

	compute.LocalScoped[bytestream.ByteStream]
}

func (f *fetch) Action() *tasks.ActionEvent {
	return tasks.Action("artifact.fetch").Arg("url", f.url)
}

func (f *fetch) Inputs() *compute.In {
	return compute.Inputs().Str("url", f.url)
}

func (f *fetch) Compute(ctx context.Context, _ compute.Resolved) (bytestream.ByteStream, error) {
	resp, err := http.Get(f.url)
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

	return bsw.Complete()
}
