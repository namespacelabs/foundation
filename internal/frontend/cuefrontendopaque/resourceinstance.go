// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/source/protos"
	"namespacelabs.dev/foundation/workspace/source/protos/fnany"
)

type cueResourceInstance struct {
	Kind string `json:"kind"`
	On   string `json:"on"`

	IntentFrom *cuefrontend.CueInvokeBinary `json:"from"`
}

func parseResourceInstance(ctx context.Context, pl workspace.EarlyPackageLoader, loc pkggraph.Location, name string, v *fncue.CueV) (*schema.ResourceInstance, error) {
	var bits cueResourceInstance
	if err := v.Val.Decode(&bits); err != nil {
		return nil, err
	}

	pkgRef, err := schema.ParsePackageRef(bits.Kind)
	if err != nil {
		return nil, err
	}

	intent, err := parseResourceIntent(ctx, pl, loc, pkgRef, v.LookupPath("input"))
	if err != nil {
		return nil, err
	}

	intentFrom, err := bits.IntentFrom.ToFrontend()
	if err != nil {
		return nil, err
	}

	return &schema.ResourceInstance{
		Name:       name,
		Class:      pkgRef,
		Provider:   bits.On,
		Intent:     intent,
		IntentFrom: intentFrom,
	}, nil
}

func parseResourceIntent(ctx context.Context, pl workspace.EarlyPackageLoader, loc pkggraph.Location, pkgRef *schema.PackageRef, v *fncue.CueV) (*anypb.Any, error) {
	if !v.Exists() {
		return nil, nil
	}

	pkg, err := pl.LoadByName(ctx, pkgRef.AsPackageName())
	if err != nil {
		return nil, err
	}
	rc := pkg.ResourceClass(pkgRef.Name)
	if rc == nil {
		return nil, fnerrors.UserError(loc, "resource class %q not found", pkgRef.Canonical())
	}

	rcFds, ok := pkg.Provides[pkgRef.Name]
	if !ok {
		return nil, fnerrors.InternalError("proto descriptors are missing for %q at %q", pkgRef.Canonical(), loc.PackageName)
	}

	_, md, err := protos.LoadMessageByName(rcFds, rc.IntentType.ProtoType)
	if err != nil {
		return nil, fnerrors.InternalError("failed to load message %q: %v", rc.IntentType.ProtoType, err)
	}

	return parseResourceIntentProto(schema.MakePackageName(rc.PackageName), v, md)
}

func parseResourceIntentProto(rcPackageName schema.PackageName, v *fncue.CueV, md protoreflect.MessageDescriptor) (*anypb.Any, error) {
	// TODO: custom parsing of "foundation.std.types.Resource" types: inlining resource files etc.
	msg, err := v.DecodeAs(dynamicpb.NewMessageType(md))
	if err != nil {
		return nil, err
	}

	return fnany.Marshal(rcPackageName, msg)
}
