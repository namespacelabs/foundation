// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/workspace"
)

type targetPkg struct {
	Loc  fnfs.Location
	Root *workspace.Root
}

func parseTargetPkgWithDeps(targetPkgOut *targetPkg, typ string) []fncobra.ArgParser {
	var (
		env  provision.Env
		locs fncobra.Locations
	)
	return []fncobra.ArgParser{
		fncobra.ParseEnv(&env),
		fncobra.ParseLocations(&locs, &fncobra.ParseLocationsOpts{RequireSingle: true}),
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
	loc := p.locs.Locs[0]
	if loc.RelPath == "." {
		cmd := fmt.Sprintf("ns create %s", p.typ)
		return fmt.Errorf(
			"cannot create %s at workspace root. Please specify %s location or run %s at the target directory",
			p.typ, p.typ, colors.Ctx(ctx).Highlight.Apply(cmd))
	}

	p.targetPkgOut.Loc = loc
	p.targetPkgOut.Root = p.locs.Root

	return nil
}
