// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"context"

	"namespacelabs.dev/foundation/internal/codegen/genpackage"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func codegenNode(ctx context.Context, out pkggraph.MutableModule, env cfg.Context, loc fnfs.Location) error {
	// Aggregates and prints all accumulated codegen errors on return.
	var errorCollector fnerrors.ErrorCollector

	// Generate protos before generating code for this extension as code (our generated code may depend on the protos).
	if err := genpackage.ForLocationsGenProto(ctx, out, env, []fnfs.Location{loc}, errorCollector.Append); err != nil {
		return err
	}

	if err := genpackage.ForLocationsGenCode(ctx, out, env, []fnfs.Location{loc}, errorCollector.Append); err != nil {
		return err
	}

	return errorCollector.Error()
}
