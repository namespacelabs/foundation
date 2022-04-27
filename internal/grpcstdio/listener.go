// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package grpcstdio

import (
	"context"
	"errors"
	"net"
	"sync"
)

var ErrListenerClosed = errors.New("listener was closed")

func NewListener(ctx context.Context) *StdinListener {
	return &StdinListener{ctx: ctx, ch: make(chan net.Conn)}
}

type StdinListener struct {
	ctx    context.Context
	ch     chan net.Conn
	mu     sync.Mutex
	closed bool
}

func (lis *StdinListener) Ready(ctx context.Context, conn net.Conn) error {
	select {
	case lis.ch <- conn:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (lis *StdinListener) Accept() (net.Conn, error) {
	select {
	case conn, ok := <-lis.ch:
		if !ok {
			return nil, ErrListenerClosed
		}
		return conn, nil
	case <-lis.ctx.Done():
		return nil, lis.ctx.Err()
	}
}

func (lis *StdinListener) Close() error {
	lis.mu.Lock()
	if !lis.closed {
		close(lis.ch)
		lis.closed = true
	}
	lis.mu.Unlock()
	return nil
}

func (lis *StdinListener) Addr() net.Addr {
	return stdioAddr{"stdio", "listener"}
}
