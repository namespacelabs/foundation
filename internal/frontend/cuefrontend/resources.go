// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"context"
	"encoding/json"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/source/protos/fnany"
)

type ResourceList struct {
	Refs      []string
	Instances map[string]CueResourceInstance
}

var _ json.Unmarshaler = &ResourceList{}

type CueResourceInstance struct {
	Class      string           `json:"kind"`
	On         string           `json:"on"`
	IntentFrom *CueInvokeBinary `json:"from"`
	Input      any              `json:"input"`
}

func ParseResourceInstanceFromCue(ctx context.Context, pl workspace.EarlyPackageLoader, loc pkggraph.Location, name string, v *fncue.CueV) (*schema.ResourceInstance, error) {
	var instance CueResourceInstance
	if err := v.Val.Decode(&instance); err != nil {
		return nil, err
	}

	return ParseResourceInstance(ctx, pl, loc, name, instance)
}

func ParseResourceInstance(ctx context.Context, pl pkggraph.PackageLoader, loc pkggraph.Location, name string, instance CueResourceInstance) (*schema.ResourceInstance, error) {
	classRef, err := schema.ParsePackageRef(instance.Class)
	if err != nil {
		return nil, err
	}

	intent, err := parseResourceIntent(ctx, pl, loc, classRef, instance.Input)
	if err != nil {
		return nil, err
	}

	intentFrom, err := instance.IntentFrom.ToInvocation()
	if err != nil {
		return nil, err
	}

	return &schema.ResourceInstance{
		Name:       name,
		Class:      classRef,
		Provider:   instance.On,
		Intent:     intent,
		IntentFrom: intentFrom,
	}, nil
}

func parseResourceIntent(ctx context.Context, pl pkggraph.PackageLoader, loc pkggraph.Location, classRef *schema.PackageRef, value any) (*anypb.Any, error) {
	if value == nil {
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

	return parseResourceIntentProto(rc.PackageName(), value, rc.IntentType.Descriptor)
}

func parseResourceIntentProto(pkg schema.PackageName, value any, intentType protoreflect.MessageDescriptor) (*anypb.Any, error) {
	serializedValue, err := json.Marshal(value)
	if err != nil {
		return nil, fnerrors.InternalError("%s: failed to serialize input: %w", pkg, err)
	}

	msg := dynamicpb.NewMessage(intentType).Interface()

	// TODO: custom parsing of "foundation.std.types.Resource" types: inlining resource files etc.
	if err := (protojson.UnmarshalOptions{}).Unmarshal(serializedValue, msg); err != nil {
		return nil, fnerrors.InternalError("%s: failed to coerce input: %w", pkg, err)
	}

	return fnany.Marshal(pkg, msg)
}

func (rl *ResourceList) UnmarshalJSON(contents []byte) error {
	var list []string
	if json.Unmarshal(contents, &list) == nil {
		rl.Refs = list
		return nil
	}

	var instances map[string]CueResourceInstance
	if err := json.Unmarshal(contents, &instances); err != nil {
		return err
	}

	rl.Instances = instances
	return nil
}

func (rl *ResourceList) ToPack(ctx context.Context, pl pkggraph.PackageLoader, loc pkggraph.Location) (*schema.ResourcePack, error) {
	pack := &schema.ResourcePack{}

	for _, resource := range rl.Refs {
		r, err := parseResourceRef(ctx, pl, loc, resource)
		if err != nil {
			return nil, err
		}

		pack.ResourceRef = append(pack.ResourceRef, r)
	}

	for name, instance := range rl.Instances {
		instance, err := ParseResourceInstance(ctx, pl, loc, name, instance)
		if err != nil {
			return nil, err
		}

		pack.ResourceInstance = append(pack.ResourceInstance, instance)
	}

	return pack, nil
}

func parseResourceRef(ctx context.Context, pl pkggraph.PackageLoader, loc pkggraph.Location, packageRef string) (*schema.PackageRef, error) {
	pkgRef, err := schema.ParsePackageRef(packageRef)
	if err != nil {
		return nil, err
	}

	pkg, err := pl.LoadByName(ctx, pkgRef.AsPackageName())
	if err != nil {
		return nil, err
	}

	r := pkg.LookupResourceInstance(pkgRef.Name)
	if r == nil {
		return nil, fnerrors.UserError(loc, "no such resource %q", pkgRef.Name)
	}

	return pkgRef, nil
}
