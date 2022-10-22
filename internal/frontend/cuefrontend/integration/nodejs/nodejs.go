// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package nodejs

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
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

	// Name -> package name.
	// The ingress urls for backends are injected into the built image as a JS file.
	Backends map[string]string `json:"backends"`
}

func (i *Parser) Parse(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, v *fncue.CueV) (proto.Message, error) {
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

	fmt.Fprintf(console.Debug(ctx), "nodejs: %s: detected package manager: %s\n", loc.Abs(), pkgMgr)

	packageJson, err := readPackageJson(ctx, pl, loc)
	if err != nil {
		return nil, err
	}

	scripts := []string{}
	for s := range packageJson.Scripts {
		scripts = append(scripts, s)
	}

	backends, err := ParseBackends(loc, bits.Backends)
	if err != nil {
		return nil, err
	}

	return &schema.NodejsIntegration{
		Pkg:                bits.Pkg,
		NodePkgMgr:         pkgMgr,
		PackageJsonScripts: scripts,
		Backend:            backends,
	}, nil
}

func detectPkgMgr(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location) (schema.NodejsIntegration_NodePkgMgr, error) {
	fsys, err := pl.WorkspaceOf(ctx, loc.Module)
	if err != nil {
		return schema.NodejsIntegration_PKG_MGR_UNKNOWN, err
	}

	if _, err := fs.Stat(fsys, filepath.Join(loc.Rel(), npmLockfile)); err == nil {
		return schema.NodejsIntegration_NPM, nil
	}
	if _, err := fs.Stat(fsys, filepath.Join(loc.Rel(), ".yarn", "releases")); err == nil {
		return schema.NodejsIntegration_YARN3, nil
	}
	if _, err := fs.Stat(fsys, filepath.Join(loc.Rel(), yarnLockfile)); err == nil {
		return schema.NodejsIntegration_YARN, nil
	}
	if _, err := fs.Stat(fsys, filepath.Join(loc.Rel(), pnpmLockfile)); err == nil {
		return schema.NodejsIntegration_PNPM, nil
	}

	return schema.NodejsIntegration_PKG_MGR_UNKNOWN, fnerrors.UserError(loc, "no package manager detected")
}

func ParseBackends(loc pkggraph.Location, src map[string]string) ([]*schema.NodejsIntegration_Backend, error) {
	backends := []*schema.NodejsIntegration_Backend{}
	for k, v := range src {
		serviceRef, err := schema.ParsePackageRef(loc.PackageName, v)
		if err != nil {
			return nil, err
		}

		backends = append(backends, &schema.NodejsIntegration_Backend{
			Name:    k,
			Service: serviceRef,
		})
	}

	return backends, nil
}
