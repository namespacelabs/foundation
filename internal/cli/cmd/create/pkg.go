// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package create

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/std/cfg"
)

type targetPkg struct {
	Location fnfs.Location
	Root     *parsing.Root
}

func parseTargetPkgWithDeps(targetPkgOut *targetPkg, typ string) []fncobra.ArgsParser {
	var (
		env  cfg.Context
		locs fncobra.Locations
	)
	return []fncobra.ArgsParser{
		fncobra.ParseEnv(&env),
		fncobra.ParseLocations(&locs, &env, fncobra.ParseLocationsOpts{RequireSingle: true}),
		&targetPkgParser{targetPkgOut, &locs, typ},
	}
}

type targetPkgParser struct {
	targetPkgOut *targetPkg
	locs         *fncobra.Locations
	typ          string
}

func (p *targetPkgParser) AddFlags(cmd *cobra.Command) {}

func (p *targetPkgParser) Parse(ctx context.Context, args []string) error {
	loc := p.locs.Locations[0]
	if loc.RelPath == "." {
		cmd := fmt.Sprintf("ns create %s", p.typ)
		return fmt.Errorf(
			"cannot create %s at workspace root. Please specify %s location or run %s at the target directory",
			p.typ, p.typ, colors.Ctx(ctx).Highlight.Apply(cmd))
	}

	p.targetPkgOut.Location = loc
	p.targetPkgOut.Root = p.locs.Root

	return nil
}
