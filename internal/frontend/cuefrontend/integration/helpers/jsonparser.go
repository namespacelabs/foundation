// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package helpers

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type SimpleJsonParser[V proto.Message] struct {
	// Not "Url" to avoid a collision with the "Url" method.
	SyntaxUrl      string
	SyntaxShortcut string
}

func (p *SimpleJsonParser[V]) Url() string      { return p.SyntaxUrl }
func (p *SimpleJsonParser[V]) Shortcut() string { return p.SyntaxShortcut }

func (p *SimpleJsonParser[V]) Parse(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, loc pkggraph.Location, v *fncue.CueV) (proto.Message, error) {
	return fncue.DecodeToTypedProtoMessage[V](v)
}
