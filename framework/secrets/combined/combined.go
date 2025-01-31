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

var secretProviders = map[string]func(context.Context, cfg.Configuration, *secrets.SecretLoadRequest, *anypb.Any) ([]byte, error){}

type SecretProvider[V proto.Message] interface {
	Load(context.Context, V) ([]byte, error)
}

func RegisterSecretsProvider[V proto.Message](handle func(context.Context, cfg.Configuration, *secrets.SecretLoadRequest, V) ([]byte, error), aliases ...string) {
	secretProviders[protos.TypeUrl[V]()] = func(ctx context.Context, conf cfg.Configuration, req *secrets.SecretLoadRequest, input *anypb.Any) ([]byte, error) {
		msg := protos.NewFromType[V]()
		if err := input.UnmarshalTo(msg); err != nil {
			return nil, err
		}

		return handle(ctx, conf, req, msg)
	}
}

type combinedSecrets struct {
	envConf cfg.Configuration
	// canonical secret ref -> secret binding
	bindings map[string]*schema.Workspace_SecretBinding
	local    secrets.SecretsSource

	mu      sync.RWMutex
	loaded  map[string][]byte         // secret ref -> value
	loading map[string]*loadingSecret // secret ref -> loadingSecret
}

type resultPair struct {
	value []byte
	err   error
}

type loadingSecret struct {
	req  *secrets.SecretLoadRequest
	load func(context.Context, cfg.Configuration, *secrets.SecretLoadRequest, *anypb.Any) ([]byte, error)
	cfg  *anypb.Any
	cs   *combinedSecrets

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
	overwrites := map[string]*schema.Workspace_SecretBinding{}
	for _, sb := range env.Workspace().Proto().SecretBinding {
		if sb.Environment == "" {
			bindings[sb.PackageRef.Canonical()] = sb
		}

		if sb.Environment == env.Environment().Name {
			overwrites[sb.PackageRef.Canonical()] = sb
		}
	}

	for key, val := range overwrites {
		bindings[key] = val
	}

	return &combinedSecrets{
		envConf:  env.Configuration(),
		bindings: bindings,
		local:    local,
		loaded:   map[string][]byte{},
		loading:  map[string]*loadingSecret{},
	}, nil
}

func (cs *combinedSecrets) Load(ctx context.Context, modules pkggraph.ModuleResolver, req *secrets.SecretLoadRequest) (*schema.SecretResult, error) {
	cs.mu.RLock()
	value := cs.loaded[req.String()]
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
		loading := cs.loading[req.String()]
		if loading == nil {
			loading = &loadingSecret{
				req:  req,
				load: p,
				cfg:  b.Configuration,
				cs:   cs,
			}
			cs.loading[req.String()] = loading
		}
		cs.mu.Unlock()

		value, err := loading.Get(ctx)
		if err != nil {
			return nil, err
		}

		if value == nil {
			return nil, nil
		}

		return &schema.SecretResult{Value: value, FileContents: &schema.FileContents{Contents: value}}, nil
	}

	return cs.local.Load(ctx, modules, req)
}

func (cs *combinedSecrets) MissingError(missing *schema.PackageRef, missingSpec *schema.SecretSpec, missingServer schema.PackageName) error {
	return cs.local.MissingError(missing, missingSpec, missingServer)
}

func (cs *combinedSecrets) complete(id *secrets.SecretLoadRequest, res []byte) {
	cs.mu.Lock()
	cs.loaded[id.String()] = res
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
	res.value, res.err = l.load(ctx, l.cs.envConf, l.req, l.cfg)
	l.mu.Lock()

	l.done = true
	l.result = res

	if res.err == nil {
		l.cs.complete(l.req, res.value)
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
