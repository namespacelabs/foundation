// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package parsing

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"namespacelabs.dev/foundation/framework/kubernetes/kubenaming"
	"namespacelabs.dev/foundation/internal/codegen/protos"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func ValidateServerID(n *schema.Server) error {
	matched, err := regexp.MatchString("^[0-9a-z]{16,32}$", n.GetId())
	if err != nil {
		return fnerrors.InternalError("unable to validate id: %w", err)
	}

	if !matched {
		return fnerrors.New("invalid id: %v", n.GetId())
	}

	return nil
}

func TransformServer(ctx context.Context, pl pkggraph.PackageLoader, srv *schema.Server, pp *pkggraph.Package) (*schema.Server, error) {
	if srv.Name == "" {
		return nil, fnerrors.NewWithLocation(pp.Location, "server name is required")
	}

	if srv.Id == "" {
		srv.Id = kubenaming.StableIDN(pp.Location.PackageName.String(), 16)
	}

	if err := ValidateServerID(srv); err != nil {
		return nil, err
	}

	if srv.Self != nil {
		if _, err := TransformServerFragment(ctx, pl, srv.Self, pp); err != nil {
			return nil, err
		}
	}

	loc := pp.Location

	srv.PackageName = loc.PackageName.String()
	srv.ModuleName = loc.Module.ModuleName()
	srv.UserImports = srv.Import

	if handler, ok := FrameworkHandlers[srv.Framework]; ok {
		var ext ServerFrameworkExt
		if err := handler.PreParseServer(ctx, loc, &ext); err != nil {
			return nil, err
		}

		if ext.FrameworkSpecific != nil {
			srv.Ext = append(srv.Ext, ext.FrameworkSpecific)
		}
	}

	s := newSealer(ctx, pl, loc.PackageName, nil)
	if err := s.DoServer(loc, srv, pp); err != nil {
		_ = s.g.Wait() // Make sure cancel is triggered.
		return nil, err
	}

	sealed, err := s.finishSealing(ctx)
	if err != nil {
		return nil, err
	}

	if err := validatePackage(ctx, pp); err != nil {
		return nil, err
	}

	var sorted schema.PackageList
	flattenNodeDeps(sealed.Result, s.serverIncludes, &sorted)
	sealed.Result.Server.Import = sorted.PackageNamesAsString()

	var ida depVisitor
	for _, dep := range sealed.Deps {
		n := dep.Node()
		if n == nil {
			continue
		}

		if n.Kind == schema.Node_SERVICE && n.ServiceFramework != srv.Framework {
			return nil, fnerrors.NewWithLocation(
				dep.Location,
				"The server '%s' can only embed services of its framework %s. Can't embed service '%s' implemented in %s.",
				srv.PackageName,
				srv.Framework,
				n.PackageName,
				n.ServiceFramework,
			)
		}

		if err := ida.visit(ctx, pl, &srv.Allocation, n, ""); err != nil {
			return nil, err
		}
	}

	if sealed.Result.Server.Self == nil {
		sealed.Result.Server.Self = &schema.ServerFragment{}
	}

	for _, dep := range sealed.Deps {
		if node := dep.Node(); node != nil {
			if err := mergeNodeLike(node, sealed.Result.Server.Self); err != nil {
				return nil, err
			}

			for _, rs := range node.GetMount() {
				if rs.Owner != node.GetPackageName() {
					return nil, fnerrors.BadInputError("%s: mount: didn't expect owner to be %q", node.PackageName, rs.Owner)
				}

				srv.Self.MainContainer.Mount = append(srv.Self.MainContainer.Mount, rs)
			}

			if node.EnvironmentRequirement != nil {
				sealed.Result.Server.EnvironmentRequirement = append(sealed.Result.Server.EnvironmentRequirement, &schema.Server_EnvironmentRequirement{
					Package:                     node.PackageName,
					EnvironmentHasLabel:         node.EnvironmentRequirement.EnvironmentHasLabel,
					EnvironmentDoesNotHaveLabel: node.EnvironmentRequirement.EnvironmentDoesNotHaveLabel,
				})
			}
		}
	}

	if handler, ok := FrameworkHandlers[srv.Framework]; ok {
		if err := handler.PostParseServer(ctx, &sealed); err != nil {
			return nil, err
		}
	}

	return sealed.Result.Server, nil
}

func TransformServerFragment(ctx context.Context, pl pkggraph.PackageLoader, frag *schema.ServerFragment, pp *pkggraph.Package) (*schema.ServerFragment, error) {
	// Make services and endpoints order stable.
	sortServices(frag.Service)
	sortServices(frag.Ingress)

	return frag, nil
}

func mergeNodeLike(node *schema.Node, out *schema.ServerFragment) error {
	for _, rs := range node.GetVolume() {
		if rs.Owner != node.GetPackageName() {
			return fnerrors.BadInputError("%s: volume: didn't expect owner to be %q", node.GetPackageName(), rs.Owner)
		}

		out.Volume = append(out.Volume, rs)
	}

	if node.GetResourcePack() != nil {
		if out.ResourcePack == nil {
			out.ResourcePack = &schema.ResourcePack{}
		}

		MergeResourcePack(node.GetResourcePack(), out.ResourcePack)
	}

	return nil
}

func MergeResourcePack(src, out *schema.ResourcePack) {
	out.ResourceInstance = append(out.ResourceInstance, src.ResourceInstance...)
	mergeResourceRefs(src, out)
}

func mergeResourceRefs(src, out *schema.ResourcePack) {
	// O(n^2)
	for _, ref := range src.ResourceRef {
		existing := false
		for _, x := range out.ResourceRef {
			if x.Canonical() == ref.Canonical() {
				existing = true
				break
			}
		}
		if !existing {
			out.ResourceRef = append(out.ResourceRef, ref)
		}
	}
}

func sortServices(services []*schema.Server_ServiceSpec) {
	sort.Slice(services, func(i, j int) bool {
		if services[i].GetPort().GetContainerPort() == services[j].GetPort().GetContainerPort() {
			return strings.Compare(services[i].Name, services[j].Name) < 0
		}
		return services[i].GetPort().GetContainerPort() < services[j].GetPort().GetContainerPort()
	})
}

func validatePackage(ctx context.Context, pp *pkggraph.Package) error {
	binaryNames := map[string]bool{}
	for _, binary := range pp.Binaries {
		if binaryNames[binary.Name] {
			return fnerrors.NewWithLocation(pp.Location, "duplicate binary name %q", binary.Name)
		}
		binaryNames[binary.Name] = true
	}

	return nil
}

type depVisitor struct{ alloc int }

func (depv *depVisitor) allocName(p string) string {
	n := depv.alloc
	depv.alloc++
	if p != "" {
		return fmt.Sprintf("%s.%d", p, n)
	}
	return fmt.Sprintf("%d", n)
}

func (depv *depVisitor) visit(ctx context.Context, pl pkggraph.PackageLoader, allocs *[]*schema.Allocation, n *schema.Node, p string) error {
	// XXX this is not quite right as one of the downstream dependencies may
	// end up performing an allocation, but keeping it simple for now.
	if len(n.Instantiate) == 0 {
		return nil
	}

	var deps schema.PackageList
	perPkg := map[schema.PackageName][]*schema.Instantiate{}

	// XXX at the moment the instantiate statements are not being dedup'd
	// but it may sense to do so in the future (i.e. two instantiate with
	// the same constructor would yield the same value).
	for _, n := range n.Instantiate {
		if ref := protos.Ref(n.Constructor); ref != nil && !ref.Builtin {
			deps.Add(ref.Package)
			perPkg[ref.Package] = append(perPkg[ref.Package], n)
		}
	}

	alloc := &schema.Allocation{}
	for _, pkg := range deps.PackageNames() {
		dep, err := loadDep(ctx, pl, pkg)
		if err != nil {
			return err
		}

		inst := &schema.Allocation_Instance{
			InstanceOwner: n.GetPackageName(),
			PackageName:   pkg.String(),
			Instantiated:  perPkg[pkg],
			AllocName:     depv.allocName(p),
		}

		if err := depv.visit(ctx, pl, &inst.DownstreamAllocation, dep.Node(), inst.AllocName); err != nil {
			return err
		}

		alloc.Instance = append(alloc.Instance, inst)
	}

	*allocs = append(*allocs, alloc)
	return nil
}
