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
	"namespacelabs.dev/foundation/workspace/source/protos/fnany"
)

type cueResourceInstance struct {
	Class string `json:"kind"`
	On    string `json:"on"`

	IntentFrom *cuefrontend.CueInvokeBinary `json:"from"`
}

func parseResourceInstance(ctx context.Context, pl workspace.EarlyPackageLoader, loc pkggraph.Location, name string, v *fncue.CueV) (*schema.ResourceInstance, error) {
	var bits cueResourceInstance
	if err := v.Val.Decode(&bits); err != nil {
		return nil, err
	}

	classRef, err := schema.ParsePackageRef(bits.Class)
	if err != nil {
		return nil, err
	}

	intent, err := parseResourceIntent(ctx, pl, loc, classRef, v.LookupPath("input"))
	if err != nil {
		return nil, err
	}

	intentFrom, err := bits.IntentFrom.ToInvocation()
	if err != nil {
		return nil, err
	}

	return &schema.ResourceInstance{
		Name:       name,
		Class:      classRef,
		Provider:   bits.On,
		Intent:     intent,
		IntentFrom: intentFrom,
	}, nil
}

func parseResourceIntent(ctx context.Context, pl workspace.EarlyPackageLoader, loc pkggraph.Location, classRef *schema.PackageRef, v *fncue.CueV) (*anypb.Any, error) {
	if !v.Exists() {
		return nil, nil
	}

	pkg, err := pl.LoadByName(ctx, classRef.AsPackageName())
	if err != nil {
		return nil, err
	}

	rc := pkg.LookupResourceClass(classRef.Name)
	if rc == nil {
		return nil, fnerrors.UserError(loc, "resource class %q not found", classRef.Canonical())
	}

	return parseResourceIntentProto(rc.PackageName(), v, rc.IntentType.Descriptor)
}

func parseResourceIntentProto(rcPackageName schema.PackageName, v *fncue.CueV, intentType protoreflect.MessageDescriptor) (*anypb.Any, error) {
	// TODO: custom parsing of "foundation.std.types.Resource" types: inlining resource files etc.
	msg, err := v.DecodeAs(dynamicpb.NewMessageType(intentType))
	if err != nil {
		return nil, err
	}

	return fnany.Marshal(rcPackageName, msg)
}
