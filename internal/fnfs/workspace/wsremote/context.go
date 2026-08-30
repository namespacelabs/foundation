// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package wsremote

import (
	"context"
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

type DepositFunc func(context.Context, *Signature, []*wscontents.FileEvent) (bool, error)

func Ctx(ctx context.Context) *SinkRegistrar {
	raw := ctx.Value(_registrarKey)
	if raw == nil {
		return nil
	}
	return raw.(*SinkRegistrar)
}

func BufferAndSinkTo(ctx context.Context, f DepositFunc) (context.Context, *SinkRegistrar) {
	r := &SinkRegistrar{f}

	newCtx := context.WithValue(ctx, _registrarKey, r)

	return newCtx, r
}

func (r *SinkRegistrar) For(sig *Signature) Sink {
	return &bufferingSink{r: r, sig: sig}
}

type bufferingSink struct {
	r   *SinkRegistrar
	sig *Signature

	mu       sync.Mutex
	buffered []*wscontents.FileEvent
}

func (s *bufferingSink) Deposit(ctx context.Context, events []*wscontents.FileEvent) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// XXX manage queue length.
	s.buffered = append(s.buffered, events...)

	deposited, err := s.r.deposit(ctx, s.sig, s.buffered)
	if err != nil {
		return false, err
	}

	if deposited {
		s.buffered = nil
	}

	return true, nil
}
