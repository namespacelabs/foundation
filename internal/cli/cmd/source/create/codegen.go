package create

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/source/codegen"
)

func codegenNode(ctx context.Context, root *workspace.Root, loc fnfs.Location) error {
	// Aggregates and prints all accumulated codegen errors on return.
	var errorCollector fnerrors.ErrorCollector

	// Generate protos before generating code for this extension as code (our generated code may depend on the protos).
	if err := codegen.ForLocationsGenProto(ctx, root, []fnfs.Location{loc}, errorCollector.Append); err != nil {
		return err
	}

	if err := codegen.ForLocationsGenCode(ctx, root, []fnfs.Location{loc}, errorCollector.Append); err != nil {
		return err
	}

	return errorCollector.Error()
}
