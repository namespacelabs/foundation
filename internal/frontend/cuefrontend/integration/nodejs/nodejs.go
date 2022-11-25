// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package nodejs

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/integrations/opaque"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

const (
	startScript  = "start"
	buildScript  = "build"
	devScript    = "dev"
	npmLockfile  = "package-lock.json"
	yarnLockfile = "yarn.lock"
	pnpmLockfile = "pnpm-lock.yaml"
)

type Parser struct{}

func (i *Parser) Url() string      { return "namespace.so/from-nodejs" }
func (i *Parser) Shortcut() string { return "nodejs" }

type cueIntegrationNodejs struct {
	Build cueIntegrationNodejsBuild `json:"build"`

	Pkg string `json:"pkg"`
}

type cueIntegrationNodejsBuild struct {
	OutDir string `json:"outDir"`
}

func (i *Parser) Parse(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, loc pkggraph.Location, v *fncue.CueV) (proto.Message, error) {
	var bits cueIntegrationNodejs
	if v != nil {
		if err := v.Val.Decode(&bits); err != nil {
			return nil, err
		}
	}

	relPath := filepath.Join(loc.Rel(), bits.Pkg)

	pkgMgr, err := detectPkgMgr(ctx, pl, loc, relPath)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(console.Debug(ctx), "nodejs: %s: detected package manager: %s\n", loc.Abs(), pkgMgr)

	packageJson, err := readPackageJson(ctx, pl, loc, relPath)
	if err != nil {
		return nil, err
	}

	scripts := []string{}
	for s := range packageJson.Scripts {
		scripts = append(scripts, s)
	}

	out := &schema.NodejsBuild{
		Pkg:        bits.Pkg,
		NodePkgMgr: pkgMgr,
	}

	if v != nil {
		if b := v.LookupPath("backends"); b.Exists() {
			it, err := b.Val.Fields()
			if err != nil {
				return nil, err
			}

			for it.Next() {
				val := &fncue.CueV{Val: it.Value()}
				backend, err := parseBackend(loc, it.Label(), val)
				if err != nil {
					return nil, err
				}

				out.InternalDoNotUseBackend = append(out.InternalDoNotUseBackend, backend)
			}
		}
	}

	if opaque.UseDevBuild(env) {
		// Existence of the "dev" script is not checked, because this code is executed during package loading,
		// and for "ns test" it happens initially with the "DEV" environment.
		out.RunScript = devScript
	} else {
		if !slices.Contains(scripts, startScript) {
			return nil, fnerrors.NewWithLocation(loc, `package.json must contain a script named '%s': it is invoked when starting the server in non-dev environments`, startScript)
		}

		out.RunScript = startScript
		out.Prod = &schema.NodejsBuild_Prod{
			BuildOutDir: bits.Build.OutDir,
			InstallDeps: true,
		}

		if slices.Contains(scripts, buildScript) {
			out.Prod.BuildScript = buildScript
		} else {
			if out.Prod.BuildOutDir != "" {
				return nil, fnerrors.NewWithLocation(loc, `package.json must contain '%s' script if 'build.outDir' is set`, buildScript)
			}
		}
	}

	return out, nil
}

func detectPkgMgr(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, relPath string) (schema.NodejsBuild_NodePkgMgr, error) {
	fsys, err := pl.WorkspaceOf(ctx, loc.Module)
	if err != nil {
		return schema.NodejsBuild_PKG_MGR_UNKNOWN, err
	}

	if _, err := fs.Stat(fsys, filepath.Join(relPath, npmLockfile)); err == nil {
		return schema.NodejsBuild_NPM, nil
	}
	if _, err := fs.Stat(fsys, filepath.Join(relPath, ".yarn", "releases")); err == nil {
		return schema.NodejsBuild_YARN3, nil
	}
	if _, err := fs.Stat(fsys, filepath.Join(relPath, yarnLockfile)); err == nil {
		return schema.NodejsBuild_YARN, nil
	}
	if _, err := fs.Stat(fsys, filepath.Join(relPath, pnpmLockfile)); err == nil {
		return schema.NodejsBuild_PNPM, nil
	}

	return schema.NodejsBuild_PKG_MGR_UNKNOWN, fnerrors.NewWithLocation(loc, "no package manager detected")
}

// Full form of the backend definition.
type cueNodejsBackend struct {
	Service string `json:"service"`
	Manager string `json:"internalDoNotUseManager"`
}

// The ingress urls for backends are injected into the built image as a JS file.
func parseBackend(loc pkggraph.Location, name string, v *fncue.CueV) (*schema.NodejsBuild_Backend, error) {
	backend := &schema.NodejsBuild_Backend{
		Name: name,
	}

	var rawServiceRef string
	if str, err := v.Val.String(); err == nil {
		rawServiceRef = str
	} else {
		var bits cueNodejsBackend
		if v != nil {
			if err := v.Val.Decode(&bits); err != nil {
				return nil, err
			}
		}

		rawServiceRef = bits.Service
		backend.Manager = bits.Manager
	}

	var err error
	backend.Service, err = schema.ParsePackageRef(loc.PackageName, rawServiceRef)
	if err != nil {
		return nil, err
	}

	return backend, nil
}
