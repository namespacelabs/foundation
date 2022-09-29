// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package helpers

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace"
)

type SimpleJsonParser[V proto.Message] struct {
	// Not "Kind" to avoid a collision with the "Kind" method.
	SyntaxKind     string
	SyntaxShortcut string
}

func (p *SimpleJsonParser[V]) Kind() string     { return p.SyntaxKind }
func (p *SimpleJsonParser[V]) Shortcut() string { return p.SyntaxShortcut }

func (p *SimpleJsonParser[V]) Parse(ctx context.Context, pl workspace.EarlyPackageLoader, loc pkggraph.Location, v *fncue.CueV) (proto.Message, error) {
	msg := protos.NewFromType[V]()
	if v != nil {
		if err := v.Val.Decode(&msg); err != nil {
			return nil, err
		}
	}

	return msg, nil
}
