// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package combined

import (
	"context"
	"sync"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/framework/secrets"
	"namespacelabs.dev/foundation/framework/secrets/localsecrets"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
)

var secretProviders = map[string]func(context.Context, *secrets.ServerRef, *anypb.Any) ([]byte, error){}

func RegisterSecretsProvider[V proto.Message](handle func(context.Context, *secrets.ServerRef, V) ([]byte, error), aliases ...string) {
	secretProviders[protos.TypeUrl[V]()] = func(ctx context.Context, srvRef *secrets.ServerRef, input *anypb.Any) ([]byte, error) {
		msg := protos.NewFromType[V]()
		if err := input.UnmarshalTo(msg); err != nil {
			return nil, err
		}

		return handle(ctx, srvRef, msg)
	}
}

type combinedSecrets struct {
	// canonical secret ref -> secret binding
	bindings map[string]*schema.Workspace_SecretBinding
	local    secrets.SecretsSource

	mu      sync.RWMutex
	loaded  map[secretIdentifier][]byte         // secret ref -> value
	loading map[secretIdentifier]*loadingSecret // secret ref -> loadingSecret
}

type resultPair struct {
	value []byte
	err   error
}

type secretIdentifier struct {
	serverRef schema.PackageName
	secretRef string
}

type loadingSecret struct {
	ref       *schema.PackageRef
	serverRef *secrets.ServerRef
	load      func(context.Context, *secrets.ServerRef, *anypb.Any) ([]byte, error)
	cfg       *anypb.Any
	cs        *combinedSecrets

	mu      sync.Mutex
	waiters []chan resultPair
	waiting int // The first waiter, will also get to the secret load.
	done    bool
	result  resultPair
}

func NewCombinedSecrets(env cfg.Context) (secrets.SecretsSource, error) {
	local, err := localsecrets.NewLocalSecrets(env)
	if err != nil {
		return nil, err
	}

	bindings := map[string]*schema.Workspace_SecretBinding{}

	for _, sb := range env.Workspace().Proto().SecretBinding {
		if sb.Environment == "" || sb.Environment == env.Environment().Name {
			bindings[sb.PackageRef.Canonical()] = sb
		}
	}

	return &combinedSecrets{
		bindings: bindings,
		local:    local,
		loaded:   map[secretIdentifier][]byte{},
		loading:  map[secretIdentifier]*loadingSecret{},
	}, nil
}

func (cs *combinedSecrets) Load(ctx context.Context, modules pkggraph.ModuleResolver, req *secrets.SecretLoadRequest) (*schema.SecretResult, error) {
	cs.mu.RLock()
	value := cs.loaded[secretIdentifier{req.Server.PackageName, req.SecretRef.Canonical()}]
	cs.mu.RUnlock()
	if value != nil {
		return &schema.SecretResult{Value: value, FileContents: &schema.FileContents{Contents: value}}, nil
	}

	if b, ok := cs.bindings[req.SecretRef.Canonical()]; ok {
		p := secretProviders[b.Configuration.TypeUrl]
		if p == nil {
			return nil, fnerrors.BadInputError("%s: no such secrets provider", b.Configuration.TypeUrl)
		}

		cs.mu.Lock()
		loading := cs.loading[secretIdentifier{req.Server.PackageName, req.SecretRef.Canonical()}]
		if loading == nil {
			loading = &loadingSecret{
				ref:       req.SecretRef,
				load:      p,
				cfg:       b.Configuration,
				cs:        cs,
				serverRef: req.Server,
			}
			cs.loading[secretIdentifier{req.Server.PackageName, req.SecretRef.Canonical()}] = loading
		}
		cs.mu.Unlock()

		value, err := loading.Get(ctx)
		if err != nil {
			return nil, err
		}

		return &schema.SecretResult{Value: value, FileContents: &schema.FileContents{Contents: value}}, nil
	}

	return cs.local.Load(ctx, modules, req)
}

func (cs *combinedSecrets) MissingError(missing *schema.PackageRef, missingSpec *schema.SecretSpec, missingServer schema.PackageName) error {
	return cs.local.MissingError(missing, missingSpec, missingServer)
}

func (cs *combinedSecrets) complete(srvRef schema.PackageName, ref *schema.PackageRef, res []byte) {
	cs.mu.Lock()
	cs.loaded[secretIdentifier{srvRef, ref.Canonical()}] = res
	cs.mu.Unlock()
}

func (l *loadingSecret) Get(ctx context.Context) ([]byte, error) {
	l.mu.Lock()

	rev := l.waiting
	l.waiting++

	if rev > 0 {
		// Someone is already loading the secret.
		if l.done {
			defer l.mu.Unlock()
			return l.result.value, l.result.err
		}

		// Very important that this is a buffered channel, else the write above will
		// block forever and deadlock secret loading.
		ch := make(chan resultPair, 1)
		l.waiters = append(l.waiters, ch)
		l.mu.Unlock()

		select {
		case v, ok := <-ch:
			if !ok {
				return nil, fnerrors.InternalError("unexpected eof")
			}
			return v.value, v.err

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	l.mu.Unlock()
	var res resultPair
	res.value, res.err = l.load(ctx, l.serverRef, l.cfg)
	l.mu.Lock()

	l.done = true
	l.result = res

	if res.err == nil {
		l.cs.complete(l.serverRef.PackageName, l.ref, res.value)
	}

	waiters := l.waiters
	l.waiters = nil
	l.mu.Unlock()

	for _, ch := range waiters {
		ch <- res
		close(ch)
	}
	return res.value, res.err
}
