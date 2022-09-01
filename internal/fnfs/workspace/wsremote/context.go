// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package wsremote

import (
	"context"
	"errors"
	"sync"

	"namespacelabs.dev/foundation/internal/wscontents"
)

type contextKey string

var (
	_registrarKey = contextKey("fn.internal.wsremote.registrar")
)

type SinkRegistrar struct {
	deposit DepositFunc
}

var ErrNotReady = errors.New("not ready")

type DepositFunc func(context.Context, *Signature, []*wscontents.FileEvent) error

func Ctx(ctx context.Context) *SinkRegistrar {
	raw := ctx.Value(_registrarKey)
	if raw == nil {
		return nil
	}
	return raw.(*SinkRegistrar)
}

func WithRegistrar(ctx context.Context, f DepositFunc) (context.Context, *SinkRegistrar) {
	r := &SinkRegistrar{f}

	newCtx := context.WithValue(ctx, _registrarKey, r)

	return newCtx, r
}

func (r *SinkRegistrar) For(sig *Signature) Sink {
	return &staticSink{r: r, sig: sig}
}

type staticSink struct {
	r   *SinkRegistrar
	sig *Signature

	mu       sync.Mutex
	buffered []*wscontents.FileEvent
}

func (s *staticSink) Deposit(ctx context.Context, events []*wscontents.FileEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// XXX manage queue length.
	s.buffered = append(s.buffered, events...)

	if err := s.r.deposit(ctx, s.sig, s.buffered); err != nil {
		if errors.Is(err, ErrNotReady) {
			return nil
		}
		return err
	}

	s.buffered = nil

	return nil
}
