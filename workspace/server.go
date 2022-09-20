// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/support/naming"
	"namespacelabs.dev/foundation/runtime/storage"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

func ValidateServerID(n *schema.Server) error {
	matched, err := regexp.MatchString("^[0-9a-z]{16,32}$", n.GetId())
	if err != nil {
		return fnerrors.InternalError("unable to validate id: %w", err)
	}

	if !matched {
		return fnerrors.UserError(nil, "invalid id: %v", n.GetId())
	}

	return nil
}

func TransformServer(ctx context.Context, pl pkggraph.PackageLoader, srv *schema.Server, pp *pkggraph.Package, opts LoadPackageOpts) (*schema.Server, error) {
	if srv.Name == "" {
		return nil, fnerrors.UserError(pp.Location, "server name is required")
	}

	if srv.Id == "" {
		srv.Id = naming.StableIDN(pp.Location.PackageName.String(), 16)
	}

	if err := ValidateServerID(srv); err != nil {
		return nil, err
	}

	loc := pp.Location

	srv.PackageName = loc.PackageName.String()
	srv.ModuleName = loc.Module.ModuleName()
	srv.UserImports = srv.Import

	// Make services and endpoints order stable.
	sortServices(srv.Service)
	sortServices(srv.Ingress)

	if handler, ok := FrameworkHandlers[srv.Framework]; ok {
		var ext ServerFrameworkExt
		if err := handler.PreParseServer(ctx, loc, &ext); err != nil {
			return nil, err
		}

		if ext.FrameworkSpecific != nil {
			srv.Ext = append(srv.Ext, ext.FrameworkSpecific)
		}
	}

	// Used by `ns tidy`.
	if !opts.LoadPackageReferences {
		return srv, nil
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

	if err := validateServer(ctx, pp.Location, srv); err != nil {
		return nil, err
	}

	if err := validatePackage(ctx, pp); err != nil {
		return nil, err
	}

	var sorted schema.PackageList
	likeTopoSort(sealed.Proto, s.serverIncludes, &sorted)
	sealed.Proto.Server.Import = sorted.PackageNamesAsString()

	var ida depVisitor
	for _, dep := range sealed.Deps {
		n := dep.Node()
		if n == nil {
			continue
		}

		if n.Kind == schema.Node_SERVICE && n.ServiceFramework != srv.Framework {
			return nil, fnerrors.UserError(
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

	persistentVolumeCount := 0
	for _, dep := range sealed.Deps {
		if node := dep.Node(); node != nil {
			for _, rs := range node.Volumes {
				if rs.Owner != node.PackageName {
					return nil, fnerrors.BadInputError("%s: volume: didn't expect owner to be %q", node.PackageName, rs.Owner)
				}

				sealed.Proto.Server.Volumes = append(sealed.Proto.Server.Volumes, rs)

				if rs.Kind == storage.VolumeKindPersistent {
					persistentVolumeCount++
				}
			}

			for _, rs := range node.Mounts {
				if rs.Owner != node.PackageName {
					return nil, fnerrors.BadInputError("%s: mount: didn't expect owner to be %q", node.PackageName, rs.Owner)
				}

				sealed.Proto.Server.MainContainer.Mounts = append(sealed.Proto.Server.MainContainer.Mounts, rs)
			}

			for _, secret := range node.Secret {
				if secret.Owner != node.PackageName {
					return nil, fnerrors.BadInputError("%s: secret: didn't expect owner to be %q", node.PackageName, secret.Owner)
				}

				sealed.Proto.Server.Secret = append(sealed.Proto.Server.Secret, secret)
			}

			if node.EnvironmentRequirement != nil {
				sealed.Proto.Server.EnvironmentRequirement = append(sealed.Proto.Server.EnvironmentRequirement, &schema.Server_EnvironmentRequirement{
					Package:                     node.PackageName,
					EnvironmentHasLabel:         node.EnvironmentRequirement.EnvironmentHasLabel,
					EnvironmentDoesNotHaveLabel: node.EnvironmentRequirement.EnvironmentDoesNotHaveLabel,
				})
			}
		}
	}

	if persistentVolumeCount > 0 && !sealed.Proto.Server.IsStateful {
		return nil, fnerrors.BadInputError("%s: servers that use persistent storage are required to set IsStateful=true",
			sealed.Proto.Server.Name)
	}

	if handler, ok := FrameworkHandlers[srv.Framework]; ok {
		if err := handler.PostParseServer(ctx, &sealed); err != nil {
			return nil, err
		}
	}

	return sealed.Proto.Server, nil
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
			return fnerrors.UserError(pp.Location, "duplicate binary name %q", binary.Name)
		}
		binaryNames[binary.Name] = true
	}

	return nil
}

func validateServer(ctx context.Context, loc pkggraph.Location, srv *schema.Server) error {
	for _, m := range srv.MainContainer.Mounts {
		if findVolume(srv.Volumes, m.VolumeName) == nil {
			return fnerrors.UserError(loc, "volume %q does not exist", m.VolumeName)
		}
	}

	volumeNames := map[string]bool{}
	for _, v := range srv.Volumes {
		if volumeNames[v.Name] {
			return fnerrors.UserError(loc, "volume %q is defined multiple times", v.Name)
		}
		volumeNames[v.Name] = true

		if v.Kind == storage.VolumeKindConfigurable {
			cv := &schema.ConfigurableVolume{}
			if err := v.Definition.UnmarshalTo(cv); err != nil {
				return fnerrors.InternalError("%s: failed to unmarshal configurable volume definition: %w", v.Name, err)
			}

			for _, e := range cv.Entries {
				if e.SecretRef != nil && e.SecretRef.Name != "" &&
					findSecret(srv.Secret, e.SecretRef.Name) == nil {
					return fnerrors.UserError(loc, "secret %q does not exist", e.SecretRef.Name)
				}
			}
		}
	}

	return nil
}

func findSecret(secrets []*schema.SecretSpec, name string) *schema.SecretSpec {
	for _, s := range secrets {
		if s.Name == name {
			return s
		}
	}
	return nil
}

func findVolume(volumes []*schema.Volume, name string) *schema.Volume {
	for _, v := range volumes {
		if v.Name == name {
			return v
		}
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
