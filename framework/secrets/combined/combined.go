// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package combined

import (
	"context"

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

var secretProviders = map[string]func(context.Context, *anypb.Any) ([]byte, error){}

func RegisterSecretsProvider[V proto.Message](handle func(context.Context, V) ([]byte, error), aliases ...string) {
	secretProviders[protos.TypeUrl[V]()] = func(ctx context.Context, input *anypb.Any) ([]byte, error) {
		msg := protos.NewFromType[V]()
		if err := input.UnmarshalTo(msg); err != nil {
			return nil, err
		}

		return handle(ctx, msg)
	}
}

type combinedSecrets struct {
	// canonical secret ref -> secret binding
	bindings map[string]*schema.Workspace_SecretBinding
	local    secrets.SecretsSource
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
	}, nil
}

func (cs *combinedSecrets) Load(ctx context.Context, modules pkggraph.ModuleResolver, req *secrets.SecretLoadRequest) (*schema.SecretResult, error) {
	if b, ok := cs.bindings[req.SecretRef.Canonical()]; ok {
		p := secretProviders[b.Configuration.TypeUrl]
		if p == nil {
			return nil, fnerrors.BadInputError("%s: no such secrets provider", b.Configuration.TypeUrl)
		}

		value, err := p(ctx, b.Configuration)
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
