// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package configure

import (
	"context"

	"namespacelabs.dev/foundation/internal/engine/ops/defs"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/schema"
)

type StackRequest struct {
	Request
	Env   *schema.Environment
	Focus *schema.Stack_Entry
	Stack *schema.Stack
}

type MakeExtension interface {
	ToDefinition() (*schema.DefExtension, error)
}

type ApplyOutput struct {
	Definitions []defs.MakeDefinition
	Extensions  []MakeExtension
}

type DeleteOutput struct {
	Ops []defs.MakeDefinition
}

type StackHandler interface {
	Apply(context.Context, StackRequest, *ApplyOutput) error
	Delete(context.Context, StackRequest, *DeleteOutput) error
}

func parseStackRequest(br Request, header *protocol.StackRelated) (StackRequest, error) {
	if header == nil {
		// This is temporary, while we move from top-level fields to {Apply,Delete} specific ones.
		header = &protocol.StackRelated{
			FocusedServer: br.r.FocusedServer,
			Env:           br.r.Env,
			Stack:         br.r.Stack,
		}
	}

	var p StackRequest

	s := header.Stack.GetServer(schema.PackageName(header.FocusedServer))
	if s == nil {
		return p, fnerrors.InternalError("%s: focused server not present in the stack", header.FocusedServer)
	}

	p.Request = br
	p.Env = header.Env
	p.Focus = s
	p.Stack = header.Stack

	return p, nil
}
