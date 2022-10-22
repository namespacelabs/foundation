// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/codegen/protos/fnany"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type ResourceList struct {
	Refs      []string
	Instances map[string]CueResourceInstance
}

var _ json.Unmarshaler = &ResourceList{}

type CueResourceInstance struct {
	Class     string            `json:"class"`
	Provider  string            `json:"provider"`
	RawIntent any               `json:"intent"`
	Resources map[string]string `json:"resources"`

	IntentFrom *CueInvokeBinary `json:"from"`

	// Prefer the definition above.
	Kind     string `json:"kind"`
	On       string `json:"on"`
	RawInput any    `json:"input"`
}

func ParseResourceInstanceFromCue(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, name string, v *fncue.CueV) (*schema.ResourceInstance, error) {
	var instance CueResourceInstance
	if err := v.Val.Decode(&instance); err != nil {
		return nil, err
	}

	return parseResourceInstance(ctx, pl, loc, name, instance)
}

func exclusiveFieldsErr(fieldName ...string) error {
	if len(fieldName) < 2 {
		return nil
	}

	var quoted []string
	for _, name := range fieldName {
		quoted = append(quoted, fmt.Sprintf("%q", name))
	}

	return fnerrors.BadInputError("%s and %s are exclusive: only one of them can be set", strings.Join(quoted[:len(quoted)-1], ", "), quoted[len(quoted)-1])
}

func parseResourceInstance(ctx context.Context, pl pkggraph.PackageLoader, loc pkggraph.Location, name string, src CueResourceInstance) (*schema.ResourceInstance, error) {
	class, err1 := parseStrFieldCompat("class", src.Class, "kind", src.Kind, true)
	provider, err2 := parseStrFieldCompat("provider", src.Provider, "on", src.On, false)
	if err := multierr.New(err1, err2); err != nil {
		return nil, err
	}

	if provider != "" && loc.PackageName != schema.PackageName(provider) {
		if _, err := pl.LoadByName(ctx, schema.PackageName(provider)); err != nil {
			return nil, err
		}
	}

	classRef, err := schema.ParsePackageRef(loc.PackageName, class)
	if err != nil {
		return nil, err
	}

	rawIntent := src.RawIntent
	if rawIntent != nil {
		if src.RawInput != nil {
			return nil, exclusiveFieldsErr("intent", "input")
		}
	} else {
		rawIntent = src.RawInput
	}

	intent, err := parseResourceIntent(ctx, pl, loc, classRef, rawIntent)
	if err != nil {
		return nil, err
	}

	intentFrom, err := src.IntentFrom.ToInvocation(loc.PackageName)
	if err != nil {
		return nil, err
	}

	instance := &schema.ResourceInstance{
		Name:       name,
		Class:      classRef,
		Provider:   provider,
		Intent:     intent,
		IntentFrom: intentFrom,
	}

	var parseErrs []error
	for key, value := range src.Resources {
		ref, err := schema.ParsePackageRef(loc.PackageName, value)
		if err != nil {
			parseErrs = append(parseErrs, err)
		} else {
			instance.InputResource = append(instance.InputResource, &schema.ResourceInstance_InputResource{
				Name:        &schema.PackageRef{PackageName: provider, Name: key},
				ResourceRef: ref,
			})
		}
	}

	slices.SortFunc(instance.InputResource, func(a, b *schema.ResourceInstance_InputResource) bool {
		x := a.Name.Compare(b.Name)
		if x == 0 {
			return strings.Compare(a.GetResourceRef().Canonical(), b.GetResourceRef().Canonical()) < 0
		}
		return x < 0
	})

	return instance, multierr.New(parseErrs...)
}

func parseStrFieldCompat(namev2, valuev2, namev1, valuev1 string, required bool) (string, error) {
	if valuev2 != "" && valuev1 != "" {
		return "", exclusiveFieldsErr(namev2, namev1)
	}

	if valuev2 != "" {
		return valuev2, nil
	}

	if valuev1 != "" {
		return valuev1, nil
	}

	if required {
		return "", fnerrors.BadInputError("a %q value required", namev2)
	}

	return "", nil
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
		instance, err := parseResourceInstance(ctx, pl, loc, name, instance)
		if err != nil {
			return nil, err
		}

		pack.ResourceInstance = append(pack.ResourceInstance, instance)
	}

	return pack, nil
}

func parseResourceRef(ctx context.Context, pl pkggraph.PackageLoader, loc pkggraph.Location, ref string) (*schema.PackageRef, error) {
	pkgRef, err := schema.ParsePackageRef(loc.PackageName, ref)
	if err != nil {
		return nil, err
	}

	if loc.PackageName != pkgRef.AsPackageName() {
		if _, err := pl.LoadByName(ctx, pkgRef.AsPackageName()); err != nil {
			return nil, err
		}
	}

	return pkgRef, nil
}
