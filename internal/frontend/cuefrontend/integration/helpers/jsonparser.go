// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package helpers

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type SimpleJsonParser[V proto.Message] struct {
	// Not "Url" to avoid a collision with the "Url" method.
	SyntaxUrl      string
	SyntaxShortcut string
}

func (p *SimpleJsonParser[V]) Url() string      { return p.SyntaxUrl }
func (p *SimpleJsonParser[V]) Shortcut() string { return p.SyntaxShortcut }

func (p *SimpleJsonParser[V]) Parse(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, v *fncue.CueV) (proto.Message, error) {
	msg := protos.NewFromType[V]()
	if v != nil {
		if err := v.Val.Decode(&msg); err != nil {
			return nil, err
		}
	}

	return msg, nil
}
