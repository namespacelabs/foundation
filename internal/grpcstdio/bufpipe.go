// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package grpcstdio

import (
	"bytes"
	"sync"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

type pipe struct {
	mu       sync.Mutex
	cond     *sync.Cond
	buf      *bytes.Buffer
	readErr  error
	writeErr error
}

type bufferedPipeReader struct {
	*pipe
}

type bufferedPipeWriter struct {
	*pipe
}

func newBufferedPipe() (*bufferedPipeReader, *bufferedPipeWriter) {
	p := &pipe{buf: bytes.NewBuffer(nil)}
	p.cond = sync.NewCond(&p.mu)
	return &bufferedPipeReader{p}, &bufferedPipeWriter{p}
}

func (r *bufferedPipeReader) Read(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for {
		if r.readErr != nil {
			return 0, r.readErr
		}

		n, _ := r.buf.Read(data)
		if n > 0 {
			return n, nil
		}

		r.cond.Wait()
	}
}

func (r *bufferedPipeReader) Close() error {
	return r.closeWithError(fnerrors.New("pipe is closed"))
}

func (r *bufferedPipeReader) closeWithError(err error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err == nil {
		panic("no error")
	}

	if r.writeErr != nil {
		return fnerrors.New("pipe is closed")
	}

	r.writeErr = err
	return nil
}

func (w *bufferedPipeWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.writeErr != nil {
		return 0, w.writeErr
	}

	n, err := w.buf.Write(data)
	w.cond.Signal()
	return n, err
}

func (w *bufferedPipeWriter) Close() error {
	return w.closeWithError(fnerrors.New("already closed"))
}

func (w *bufferedPipeWriter) closeWithError(err error) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err == nil {
		panic("no error")
	}

	if w.readErr != nil {
		return fnerrors.New("already closed")
	}

	w.readErr = err
	w.cond.Broadcast() // Wake up all readers.
	return nil
}
