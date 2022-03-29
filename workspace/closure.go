// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

// XXX rethink this.
var StaticDeps []schema.PackageName

type Sealed struct {
	Location      Location
	Proto         *schema.Stack_Entry
	FileDeps      []string
	Deps          []*Package
	ParsedPackage *Package
}

type SealHelper struct {
	AdditionalServerDeps func() ([]schema.PackageName, error)
}

func Seal(ctx context.Context, loader Packages, focus schema.PackageName, helper *SealHelper) (Sealed, error) {
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
	loader Packages
	helper *SealHelper

	mu             sync.Mutex
	seen           schema.PackageList
	result         *schema.Stack_Entry
	parsed         []*Package
	serverPackage  *Package
	serverIncludes []schema.PackageName
}

func (g *sealer) DoServer(loc Location, srv *schema.Server, pp *Package) error {
	var include []schema.PackageName

	if handler, ok := FrameworkHandlers[srv.Framework]; ok {
		var ext FrameworkExt
		if err := handler.PreParseServer(g.gctx, loc, &ext); err != nil {
			return err
		}

		include = append(include, ext.Include...)
	}

	if g.helper != nil && g.helper.AdditionalServerDeps != nil {
		deps, err := g.helper.AdditionalServerDeps()
		if err != nil {
			return err
		}

		include = append(include, deps...)
	}

	include = append(include, srv.GetImportedPackages()...)

	g.Do(include...)

	// Make sure that any static dependencies are always loaded.
	g.Do(StaticDeps...)

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.result.Server != nil {
		return fnerrors.InternalError("%v: server already defined (%v)", srv.PackageName, g.result.Server.PackageName)
	}

	g.result.Server = srv
	g.serverPackage = pp
	g.serverIncludes = include

	return nil
}

func (g *sealer) DoNode(loc Location, n *schema.Node, parsed *Package) error {
	g.Do(n.GetImportedPackages()...)

	g.mu.Lock()
	defer g.mu.Unlock()

	g.result.Node = append(g.result.Node, n)
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
				return fnerrors.UserError(pkg, "expected definition")
			}

			if p.Server != nil {
				return g.DoServer(p.Location, p.Server, p)
			} else if n := p.Node(); n != nil {
				return g.DoNode(p.Location, n, p)
			} else if p.Binary != nil || p.Test != nil {
				return nil // Nothing to do.
			} else {
				return fnerrors.UserError(pkg, "no server, and no node?")
			}
		})
	}
}

func newSealer(ctx context.Context, loader Packages, focus schema.PackageName, helper *SealHelper) *sealer {
	g, gctx := errgroup.WithContext(ctx)

	return &sealer{
		g:      g,
		gctx:   gctx,
		focus:  focus,
		loader: loader,
		helper: helper,
		result: &schema.Stack_Entry{},
	}
}

func likeTopoSort(entry *schema.Stack_Entry, pkgs []schema.PackageName, out *schema.PackageList) {
	for _, pkg := range pkgs {
		likeTopoSort(entry, entry.ImportsOf(pkg), out)
		out.Add(pkg)
	}
}

func (s *sealer) finishSealing(ctx context.Context) (Sealed, error) {
	if err := s.g.Wait(); err != nil {
		return Sealed{}, err
	}

	var nodes []*Package

	for _, n := range s.result.ExtsAndServices() {
		var parsed *Package
		for _, pp := range s.parsed {
			if pp.Location.PackageName.Equals(n.PackageName) {
				parsed = pp
				break
			}
		}

		if parsed == nil {
			return Sealed{}, fnerrors.UserError(nil, "%v: missing parsed package", n.PackageName)
		}

		nodes = append(nodes, parsed)
	}

	m := map[string]int{}

	stackEntry := s.result
	for k, pkg := range stackEntry.ImportsOf(s.focus) {
		m[pkg.String()] = k + 1
	}

	sort.Slice(stackEntry.Node, func(i, j int) bool {
		pkgI, pkgJ := stackEntry.Node[i].PackageName, stackEntry.Node[j].PackageName
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
		Proto:         stackEntry,
		Deps:          nodes,
		ParsedPackage: s.serverPackage,
	}

	return res, nil
}