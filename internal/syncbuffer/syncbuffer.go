// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package syncbuffer

import (
	"bytes"
	"io"
	"sync"

	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

type ByteBuffer struct {
	mu   sync.RWMutex
	cond *sync.Cond
	buf  bytes.Buffer
}

type byteBufferReader struct {
	sb  *ByteBuffer
	off int // The reader is not thread-safe.
}

type Writer interface {
	io.Writer
	GuaranteedWrite([]byte)
}

var Discard = discard{io.Discard}

func Seal(b []byte) *Sealed {
	return &Sealed{b}
}

func NewByteBuffer() *ByteBuffer {
	x := &ByteBuffer{}
	x.cond = sync.NewCond(x.mu.RLocker())
	return x
}

func (sb *ByteBuffer) Writer() Writer {
	return sb
}

func (sb *ByteBuffer) Reader() io.ReadCloser {
	return &byteBufferReader{sb: sb, off: 0}
}

func (sb *ByteBuffer) SharedSnapshot() []byte {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return slices.Clone(sb.buf.Bytes())
}

func (sb *ByteBuffer) Seal() *Sealed {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return &Sealed{sb.buf.Bytes()}
}

func (sb *ByteBuffer) GuaranteedWrite(p []byte) {
	// Writes to a memory buffer never fail.
	_, _ = sb.Write(p)
}

func (sb *ByteBuffer) Write(p []byte) (int, error) {
	sb.mu.Lock()
	n, err := sb.buf.Write(p)
	sb.cond.Broadcast()
	sb.mu.Unlock()
	return n, err
}

func (sb *ByteBuffer) readAt(off int, p []byte) (int, error) {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	for {
		// If there are any bytes available to read, read those immediately.
		b := sb.buf.Bytes()
		if off < len(b) {
			n := len(b) - off
			if n > len(p) {
				n = len(p)
			}
			copy(p[:n], b[off:(off+n)])
			return n, nil
		} else {
			sb.cond.Wait()
		}
	}
}

func (r *byteBufferReader) Read(p []byte) (int, error) {
	n, err := r.sb.readAt(r.off, p)
	r.off += n
	return n, err
}

func (r *byteBufferReader) Close() error { return nil }

type Sealed struct {
	finalized []byte
}

func (s *Sealed) Writer() Writer {
	return failedWriter{}
}

func (s *Sealed) Reader() io.ReadCloser {
	return io.NopCloser(bytes.NewReader(s.finalized))
}

func (s *Sealed) SharedSnapshot() []byte {
	return s.finalized
}

func (s *Sealed) Bytes() []byte { return s.finalized }

type failedWriter struct{}

func (failedWriter) GuaranteedWrite(p []byte) {
	panic("already sealed")
}

func (failedWriter) Write(p []byte) (int, error) {
	return 0, fnerrors.New("already sealed")
}

type discard struct{ io.Writer }

func (discard) GuaranteedWrite(p []byte) {}
