// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package entity

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type EntityParser interface {
	// For example, "namespace.so/from-dockerfile"
	Url() string

	// Shortcut for "kind", for example, "docker"
	Shortcut() string

	// "v" is nil if the user used the shortest syntactic form. Example: `integration: "golang"`
	Parse(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, v *fncue.CueV) (proto.Message, error)
}
