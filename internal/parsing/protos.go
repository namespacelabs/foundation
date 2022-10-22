// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package parsing

import (
	"context"
	"io/fs"

	"namespacelabs.dev/foundation/internal/codegen/protos"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func MakeProtoParseOpts(ctx context.Context, p pkggraph.PackageLoader, workspace *schema.Workspace) (protos.ParseOpts, error) {
	opts := protos.ParseOpts{}

	for _, w := range workspace.ExperimentalProtoModuleImports {
		loc, err := p.Resolve(ctx, schema.PackageName(w.ModuleName))
		if err != nil {
			return protos.ParseOpts{}, err
		}

		opts.KnownModules = append(opts.KnownModules, struct {
			ModuleName string
			FS         fs.FS
		}{
			ModuleName: loc.Module.ModuleName(),
			FS:         loc.Module.ReadOnlyFS(),
		})
	}

	return opts, nil
}
