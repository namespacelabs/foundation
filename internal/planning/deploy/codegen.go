// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deploy

import (
	"context"

	"namespacelabs.dev/foundation/internal/codegen/genpackage"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func codegenServer(ctx context.Context, srv planning.Server) error {
	// XXX we should be able to disable codegen for pure builds.
	if srv.Module().IsExternal() {
		return nil
	}

	codegen, err := genpackage.ForServerAndDeps(ctx, srv)
	if err != nil {
		return err
	}

	if len(codegen) == 0 {
		return nil
	}

	r := execution.NewPlan(codegen...)

	return execution.Execute(ctx, "workspace.codegen", r, nil,
		execution.FromContext(srv.SealedContext()),
		pkggraph.MutableModuleInjection.With(srv.Module()),
		pkggraph.PackageLoaderInjection.With(srv.SealedContext()),
	)
}
