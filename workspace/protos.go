// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"context"
	"io/fs"

	"namespacelabs.dev/foundation/workspace/source/protos"
)

func MakeProtoParseOpts(ctx context.Context, p Packages) (protos.ParseOpts, error) {
	const hardcoded = "namespacelabs.dev/foundation"

	loc, err := p.Resolve(ctx, hardcoded)
	if err != nil {
		return protos.ParseOpts{}, err
	}

	opts := protos.ParseOpts{}
	opts.KnownModules = append(opts.KnownModules, struct {
		ModuleName string
		FS         fs.FS
	}{
		ModuleName: loc.Module.ModuleName(),
		FS:         loc.Module.ReadOnlyFS(),
	})
	return opts, nil
}
