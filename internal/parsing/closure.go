// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package parsing

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"

	"golang.org/x/exp/slices"
	"golang.org/x/sync/errgroup"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

var ExtendServerHook []func(pkggraph.Location, *schema.Server) ExtendServerHookResult
var ExtendNodeHook []func(context.Context, pkggraph.PackageLoader, pkggraph.Location, *schema.Node) (*ExtendNodeHookResult, error)

type ExtendServerHookResult struct {
	Import []schema.PackageName
}

type ExtendNodeHookResult struct {
	Import       []schema.PackageName
	LoadPackages []schema.PackageName // Packages to also be loaded by nodes, but that won't be listed as dependencies.
}

type Sealed struct {
	Location      pkggraph.Location
	Result        *SealerResult
	FileDeps      []string
	Deps          []*pkggraph.Package
	ParsedPackage *pkggraph.Package
}

type SealHelper struct {
	AdditionalServerDeps func(schema.Framework) ([]schema.PackageName, error)
}

func Seal(ctx context.Context, loader pkggraph.PackageLoader, focus schema.PackageName, helper *SealHelper) (Sealed, error) {
	sealer := newSealer(ctx, loader, focus, helper)

	sealer.Do(focus)

	return sealer.finishSealing(ctx)
}

func (s Sealed) HasDep(packageName schema.PackageName) bool {
	for _, dep := range s.Deps {
		if dep.Location.PackageName == packageName {
			return true
		}
	}

	return false
}

type sealer struct {
	g      *errgroup.Group
	gctx   context.Context
	focus  schema.PackageName
	loader pkggraph.PackageLoader
	helper *SealHelper

	mu             sync.Mutex
	seen           schema.PackageList
	result         *SealerResult
	parsed         []*pkggraph.Package
	serverPackage  *pkggraph.Package
	serverIncludes []schema.PackageName
}

type SealerResult struct {
	Server          *schema.Server
	Nodes           []*schema.Node
	ServerFragments []*schema.ServerFragment
}

func (se *SealerResult) ExtsAndServices() []*schema.Node {
	return se.Nodes
}

func (se *SealerResult) ImportsOf(pkg schema.PackageName) []schema.PackageName {
	for _, n := range se.ExtsAndServices() {
		if pkg.Equals(n.GetPackageName()) {
			return schema.PackageNames(n.GetImport()...)
		}
	}

	if pkg.Equals(se.Server.GetPackageName()) {
		return schema.PackageNames(se.Server.GetImport()...)
	}

	return nil
}

func (g *sealer) DoServer(loc pkggraph.Location, srv *schema.Server, pp *pkggraph.Package) error {
	var include []schema.PackageName

	if handler, ok := FrameworkHandlers[srv.Framework]; ok {
		var ext ServerFrameworkExt
		if err := handler.PreParseServer(g.gctx, loc, &ext); err != nil {
			return err
		}

		include = append(include, ext.Include...)
	}

	if g.helper != nil && g.helper.AdditionalServerDeps != nil {
		deps, err := g.helper.AdditionalServerDeps(srv.Framework)
		if err != nil {
			return err
		}

		include = append(include, deps...)
	}

	for _, hook := range ExtendServerHook {
		r := hook(loc, srv)
		include = append(include, r.Import...)
	}

	include = append(include, srv.GetImportedPackages()...)

	g.Do(include...)

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.result.Server != nil {
		return fnerrors.InternalError("%v: server already defined (%v)", srv.PackageName, g.result.Server.PackageName)
	}

	g.result.Server = srv
	if srv.Self != nil {
		g.result.ServerFragments = append(g.result.ServerFragments, srv.Self)
	}
	g.serverPackage = pp
	g.serverIncludes = include

	return nil
}

func (g *sealer) DoNode(loc pkggraph.Location, n *schema.Node, parsed *pkggraph.Package) error {
	g.Do(n.GetImportedPackages()...)

	g.mu.Lock()
	defer g.mu.Unlock()

	g.result.Nodes = append(g.result.Nodes, n)
	g.parsed = append(g.parsed, parsed)

	return nil
}

func (g *sealer) DoServerFragment(loc pkggraph.Location, n *schema.ServerFragment, parsed *pkggraph.Package) error {
	g.Do(schema.PackageNames(n.Extension...)...)

	g.mu.Lock()
	defer g.mu.Unlock()

	g.result.ServerFragments = append(g.result.ServerFragments, n)
	g.parsed = append(g.parsed, parsed)

	return nil
}

func (g *sealer) Do(pkgs ...schema.PackageName) {
	var todo []schema.PackageName

	g.mu.Lock()
	for _, pkg := range pkgs {
		if g.seen.Add(pkg) {
			todo = append(todo, pkg)
		}
	}
	g.mu.Unlock()

	for _, pkg := range todo {
		pkg := pkg // close pkg

		g.g.Go(func() error {
			p, err := g.loader.LoadByName(g.gctx, pkg)
			if err != nil {
				return err
			}

			if p == nil {
				return fnerrors.NewWithLocation(pkg, "expected definition")
			}

			switch {
			case p.Server != nil:
				return g.DoServer(p.Location, p.Server, p)
			case p.Node() != nil:
				return g.DoNode(p.Location, p.Node(), p)
			case p.ServerFragment != nil:
				return g.DoServerFragment(p.Location, p.ServerFragment, p)
			default:
				// Do nothing.
				return nil
			}
		})
	}
}

func newSealer(ctx context.Context, loader pkggraph.PackageLoader, focus schema.PackageName, helper *SealHelper) *sealer {
	g, gctx := errgroup.WithContext(ctx)

	return &sealer{
		g:      g,
		gctx:   gctx,
		focus:  focus,
		loader: loader,
		helper: helper,
		result: &SealerResult{},
	}
}

func flattenNodeDeps(entry *SealerResult, pkgs []schema.PackageName, out *schema.PackageList) {
	for _, pkg := range pkgs {
		flattenNodeDeps(entry, entry.ImportsOf(pkg), out)
		out.Add(pkg)
	}
}

func (s *sealer) finishSealing(ctx context.Context) (Sealed, error) {
	if err := s.g.Wait(); err != nil {
		return Sealed{}, err
	}

	slices.SortFunc(s.result.Nodes, func(a, b *schema.Node) bool {
		return strings.Compare(a.PackageName, b.PackageName) < 0
	})

	slices.SortFunc(s.result.ServerFragments, func(a, b *schema.ServerFragment) bool {
		return strings.Compare(a.PackageName, b.PackageName) < 0
	})

	var deps []*pkggraph.Package
	for _, n := range s.result.ExtsAndServices() {
		var parsed *pkggraph.Package
		for _, pp := range s.parsed {
			if pp.Location.PackageName.Equals(n.PackageName) {
				parsed = pp
				break
			}
		}

		if parsed == nil {
			return Sealed{}, fnerrors.New("%v: missing parsed package", n.PackageName)
		}

		deps = append(deps, parsed)
	}

	m := map[string]int{}

	stackEntry := s.result
	for k, pkg := range stackEntry.ImportsOf(s.focus) {
		m[pkg.String()] = k + 1
	}

	sort.Slice(stackEntry.Nodes, func(i, j int) bool {
		pkgI, pkgJ := stackEntry.Nodes[i].PackageName, stackEntry.Nodes[j].PackageName
		a, ok1 := m[pkgI]
		b, ok2 := m[pkgJ]

		if !ok1 {
			a = math.MaxInt32
		}
		if !ok2 {
			b = math.MaxInt32
		}

		if a == b {
			return strings.Compare(pkgI, pkgJ) < 0
		}

		return a < b
	})

	res := Sealed{
		Result:        stackEntry,
		Deps:          deps,
		ParsedPackage: s.serverPackage,
	}

	return res, nil
}
