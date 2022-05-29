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

// DownloadUrl returns computable fetching a given url into ByteStream.
func DownloadUrl(url string) compute.Computable[bytestream.ByteStream] {
	return &downloadUrl{url: url}
}

type downloadUrl struct {
	url string

	compute.LocalScoped[bytestream.ByteStream]
}

func (d *downloadUrl) Action() *tasks.ActionEvent {
	return tasks.Action("artifact.download").Arg("url", d.url)
}

func (d *downloadUrl) Inputs() *compute.In {
	return compute.Inputs().Str("url", d.url)
}

func (d *downloadUrl) Compute(ctx context.Context, _ compute.Resolved) (bytestream.ByteStream, error) {
	resp, err := http.Get(d.url)
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
