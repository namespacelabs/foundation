// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"context"
	"io/fs"
	"path/filepath"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace"
)

const (
	npmLockfile  = "package-lock.json"
	yarnLockfile = "yarn.lock"
	pnpmLockfile = "pnpm-lock.yaml"
)

type Parser struct{}

func (i *Parser) Url() string      { return "namespace.so/from-nodejs" }
func (i *Parser) Shortcut() string { return "nodejs" }

type cueIntegrationNodejs struct {
	Pkg string `json:"pkg"`
}

func (i *Parser) Parse(ctx context.Context, pl workspace.EarlyPackageLoader, loc pkggraph.Location, v *fncue.CueV) (proto.Message, error) {
	var bits cueIntegrationNodejs
	if v != nil {
		if err := v.Val.Decode(&bits); err != nil {
			return nil, err
		}
	}

	pkgMgr, err := detectPkgMgr(ctx, pl, loc)
	if err != nil {
		return nil, err
	}

	packageJson, err := readPackageJson(ctx, pl, loc)
	if err != nil {
		return nil, err
	}

	scripts := []string{}
	for s := range packageJson.Scripts {
		scripts = append(scripts, s)
	}

	return &schema.NodejsIntegration{
		Pkg:                bits.Pkg,
		NodePkgMgr:         pkgMgr,
		PackageJsonScripts: scripts,
	}, nil
}

func detectPkgMgr(ctx context.Context, pl workspace.EarlyPackageLoader, loc pkggraph.Location) (schema.NodejsIntegration_NodePkgMgr, error) {
	fsys, err := pl.WorkspaceOf(ctx, loc.Module)
	if err != nil {
		return schema.NodejsIntegration_PKG_MGR_UNKNOWN, err
	}

	if _, err := fs.Stat(fsys, filepath.Join(loc.Rel(), npmLockfile)); err == nil {
		return schema.NodejsIntegration_NPM, nil
	}
	if _, err := fs.Stat(fsys, filepath.Join(loc.Rel(), yarnLockfile)); err == nil {
		return schema.NodejsIntegration_YARN, nil
	}
	if _, err := fs.Stat(fsys, filepath.Join(loc.Rel(), pnpmLockfile)); err == nil {
		return schema.NodejsIntegration_PNPM, nil
	}

	return schema.NodejsIntegration_PKG_MGR_UNKNOWN, fnerrors.UserError(loc, "no package manager detected")
}
