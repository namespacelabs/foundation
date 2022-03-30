// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package opaque

import (
	"context"

	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

func Register() {
	languages.Register(schema.Framework_OPAQUE, impl{})
	runtime.RegisterSupport(schema.Framework_OPAQUE, impl{})
}

type impl struct {
	languages.MaybeGenerate
	languages.MaybeTidy
	languages.NoDev
}

func (impl) PrepareBuild(ctx context.Context, _ languages.Endpoints, server provision.Server, isFocus bool) (build.Spec, error) {
	bin := server.Proto().GetBinary()
	if bin.GetPackageName() != "" {
		pkg, err := server.Env().LoadByName(ctx, schema.PackageName(bin.GetPackageName()))
		if err != nil {
			return nil, err
		}

		prep, err := binary.Plan(ctx, pkg, binary.BuildImageOpts{UsePrebuilts: true})
		if err != nil {
			return nil, err
		}

		return prep.Plan.Spec, nil
	}

	image := bin.GetPrebuilt()
	if image == "" {
		return nil, fnerrors.UserError(server.Location, "neither binary nor binary.image is set")
	}

	imgid, err := oci.ParseImageID(image)
	if err != nil {
		return nil, err
	}

	return build.PrebuiltPlan(imgid, false), nil
}

func (impl) PrepareRun(ctx context.Context, server provision.Server, run *runtime.ServerRunOpts) error {
	bin := server.Proto().GetBinary()
	if bin.GetPackageName() != "" {
		pkg, err := server.Env().LoadByName(ctx, schema.PackageName(bin.GetPackageName()))
		if err != nil {
			return err
		}

		if cmd := binary.Command(pkg); cmd != "" {
			run.Command = []string{cmd}
		}
	}

	return nil
}

func (impl) ParseNode(ctx context.Context, loc workspace.Location, ext *workspace.FrameworkExt) error {
	return nil
}

func (impl) PreParseServer(ctx context.Context, loc workspace.Location, ext *workspace.FrameworkExt) error {
	return nil
}

func (impl) PostParseServer(ctx context.Context, _ *workspace.Sealed) error {
	return nil
}

func (impl) InjectService(loc workspace.Location, node *schema.Node, svc *workspace.CueService) error {
	return nil
}

func (impl) FillEndpoint(*schema.Node, *schema.Endpoint) error {
	return nil
}

func (impl) InternalEndpoints(*schema.Environment, *schema.Server, []*schema.Endpoint_Port) ([]*schema.InternalEndpoint, error) {
	return nil, nil
}
