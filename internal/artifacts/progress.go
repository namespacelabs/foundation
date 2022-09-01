// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package artifacts

import (
	"bytes"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/dustin/go-humanize"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type ProgressReader interface {
	io.ReadCloser
	tasks.ActionProgress
}

type ProgressWriter interface {
	io.WriteCloser
	tasks.ActionProgress

	WrapReader(io.ReadCloser) io.ReadCloser
	WrapBytesAsReader([]byte) io.ReadCloser
}

type progressWriter struct {
	contentLength uint64
	closer        func() error
	total         uint64
}

type progressReader struct {
	pw     *progressWriter
	reader io.ReadCloser
}

func NewProgressReader(r io.ReadCloser, contentLength uint64) ProgressReader {
	return &progressReader{
		pw:     &progressWriter{contentLength: contentLength, closer: r.Close},
		reader: r,
	}
}

func NewProgressWriter(contentLength uint64, closer func() error) ProgressWriter {
	return &progressWriter{contentLength: contentLength, closer: closer}
}

func (p *progressWriter) Write(buf []byte) (n int, err error) {
	p.Update(uint64(len(buf)))
	return len(buf), nil
}

func (p *progressWriter) Update(written uint64) {
	atomic.AddUint64(&p.total, written)
}

func (p *progressWriter) Close() error {
	if p.closer != nil {
		return p.closer()
	}
	return nil
}

func (p *progressWriter) FormatProgress() string {
	v := atomic.LoadUint64(&p.total)
	cl := p.contentLength

	if cl == 0 {
		return humanize.Bytes(v)
	}

	percent := float64(v) / float64(cl) * 100

	return fmt.Sprintf("%.2f%% (%s of %s)", percent, humanize.Bytes(v), humanize.Bytes(cl))
}

func (p *progressWriter) WrapReader(r io.ReadCloser) io.ReadCloser {
	return &progressReader{pw: p, reader: r}
}

func (p *progressWriter) WrapBytesAsReader(content []byte) io.ReadCloser {
	return p.WrapReader(io.NopCloser(bytes.NewReader(content)))
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.reader.Read(buf)
	p.pw.Update(uint64(n))
	return n, err
}

func (p *progressReader) Close() error {
	return p.pw.Close()
}

func (p *progressReader) FormatProgress() string {
	return p.pw.FormatProgress()
}
