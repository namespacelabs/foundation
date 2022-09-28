// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package api

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type IntegrationParser interface {
	// For example, "namespace.so/from-dockerfile"
	Kind() string

	// Shortcut for "kind", for example, "docker"
	Shortcut() string

	// "integration" is nil if the user used the shortest syntactic form: `integration: "golang"`
	Parse(ctx context.Context, loc pkggraph.Location, integration *fncue.CueV) (proto.Message, error)
}
