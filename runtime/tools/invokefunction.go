// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tools

import (
	"bytes"
	"context"

	"google.golang.org/protobuf/encoding/protojson"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/sdk/deno"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/types"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const (
	invocationProtocol = "namespacelabs.dev/foundation/std/protocol/invocation"
	denoRuntime        = "namespacelabs.dev/foundation/std/experimental/deno"
)

func InvokeFunction(ctx context.Context, loc pkggraph.Location, rootDir string, inv *types.DeferredInvocation) (compute.Computable[*protocol.InvokeResponse], error) {
	if inv.ExperimentalFunction.Kind != invocationProtocol {
		return nil, fnerrors.BadInputError("unsupported protocol, expected %q got %q", invocationProtocol, inv.ExperimentalFunction.Kind)
	}

	if inv.WithInput != nil {
		return nil, fnerrors.InternalError("%s: invoke function does not support arbitrary inputs", inv.ExperimentalFunction.Kind)
	}

	switch inv.ExperimentalFunction.Runtime {
	case denoRuntime:
		d, err := deno.SDK(ctx)
		if err != nil {
			return nil, fnerrors.New("failed to invoke deno: %w", err)
		}

		return &invokeDeno{
			deno:    d,
			request: &protocol.InvokeRequest{},
			rootDir: rootDir,
			source:  loc.Abs(inv.ExperimentalFunction.Source),
		}, nil

	default:
		return nil, fnerrors.BadInputError("%s: unsupported runtime", inv.ExperimentalFunction.Runtime)
	}
}

type invokeDeno struct {
	deno    compute.Computable[deno.Deno]
	request *protocol.InvokeRequest
	rootDir string
	source  string

	compute.LocalScoped[*protocol.InvokeResponse]
}

func (inv *invokeDeno) Action() *tasks.ActionEvent {
	return tasks.Action("deno.invocation").Arg("rootDir", inv.rootDir).Arg("source", inv.source)
}

func (inv *invokeDeno) Inputs() *compute.In {
	return compute.Inputs().Computable("deno", inv.deno).Proto("request", inv.request).Indigestible("rootDir", inv.rootDir).Indigestible("source", inv.source)
}

func (inv *invokeDeno) Output() compute.Output {
	return compute.Output{NotCacheable: true}
}

func (inv *invokeDeno) Compute(ctx context.Context, deps compute.Resolved) (*protocol.InvokeResponse, error) {
	d := compute.MustGetDepValue(deps, inv.deno, "deno")

	requestBytes, err := protojson.Marshal(inv.request)
	if err != nil {
		return nil, err
	}

	// Pre-cache the available imports to modules, so we can later on make sure that no new downloads are made.
	if err := d.CacheImports(ctx, inv.rootDir); err != nil {
		return nil, err
	}

	var out bytes.Buffer
	rio := rtypes.IO{
		Stdin:  bytes.NewReader(requestBytes),
		Stdout: &out,
		Stderr: console.Output(ctx, "deno"),
	}

	if err := d.Run(ctx, inv.rootDir, rio, "run", "--cached-only", "--import-map=std/experimental/deno/import_map.json", inv.source); err != nil {
		return nil, err
	}

	response := &protocol.InvokeResponse{}
	if err := protojson.Unmarshal(out.Bytes(), response); err != nil {
		return nil, err
	}

	return response, nil
}
