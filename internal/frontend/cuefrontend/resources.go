// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"

	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/codegen/protos/fnany"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/binary"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type ResourceList struct {
	Refs      []string
	Instances map[string]*fncue.CueV
}

type CueResourceInstance struct {
	Class     string            `json:"class"`
	Provider  string            `json:"provider"`
	RawIntent any               `json:"intent"`
	Resources map[string]string `json:"resources"`

	// Prefer the definition above.
	Kind     string `json:"kind"`
	On       string `json:"on"`
	RawInput any    `json:"input"`
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

func ParseResourceInstanceFromCue(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, pkg *pkggraph.Package, name string, v *fncue.CueV) (*schema.ResourceInstance, error) {
	var src CueResourceInstance
	if err := v.Val.Decode(&src); err != nil {
		return nil, err
	}

	class, err1 := parseStrFieldCompat("class", src.Class, "kind", src.Kind, true)
	provider, err2 := parseStrFieldCompat("provider", src.Provider, "on", src.On, false)
	if err := multierr.New(err1, err2); err != nil {
		return nil, err
	}

	if provider != "" && pkg.PackageName() != schema.PackageName(provider) {
		if _, err := pl.LoadByName(ctx, schema.PackageName(provider)); err != nil {
			return nil, err
		}
	}

	classRef, err := schema.ParsePackageRef(pkg.PackageName(), class)
	if err != nil {
		return nil, err
	}

	intentFrom, err := binary.ParseBinaryInvocationField(ctx, env, pl, pkg, "genb-res-from-"+name /* binaryName */, "from" /* cuePath */, v)
	if err != nil {
		return nil, err
	}

	instance := &schema.ResourceInstance{
		Name:       name,
		Class:      classRef,
		Provider:   provider,
		IntentFrom: intentFrom,
	}

	rawIntent := src.RawIntent
	if rawIntent != nil {
		if src.RawInput != nil {
			return nil, exclusiveFieldsErr("intent", "input")
		}
	} else {
		rawIntent = src.RawInput
	}

	if intentFrom == nil {
		// Returns an empty proto of the intent type, wrapper into an Any, if rawIntent is nil.
		instance.Intent, err = parseResourceIntent(ctx, pl, pkg.Location, classRef, rawIntent)
		if err != nil {
			return nil, err
		}
	} else if rawIntent != nil {
		return nil, fnerrors.NewWithLocation(pkg.Location, "resource instance %q cannot specify both \"intent\" and \"from\"", name)
	}

	var parseErrs []error
	for key, value := range src.Resources {
		ref, err := schema.ParsePackageRef(pkg.PackageName(), value)
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

func parseResourceIntent(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, classRef *schema.PackageRef, value any) (*anypb.Any, error) {
	pkg, err := pl.LoadByName(ctx, classRef.AsPackageName())
	if err != nil {
		return nil, err
	}

	rc := pkg.LookupResourceClass(classRef.Name)
	if rc == nil {
		return nil, fnerrors.NewWithLocation(loc, "resource class %q not found", classRef.Canonical())
	}

	if value == nil {
		return nil, nil
	}

	fsys, err := pl.WorkspaceOf(ctx, loc.Module)
	if err != nil {
		return nil, fnerrors.InternalError("failed to retrieve workspace access: %w", err)
	}

	subFsys, err := fs.Sub(fsys, loc.Rel())
	if err != nil {
		return nil, fnerrors.InternalError("failed to retrieve workspace access: %w", err)
	}

	msg, err := allocateMessage(subFsys, rc.IntentType.Descriptor, value)
	if err != nil {
		return nil, err
	}

	return fnany.Marshal(rc.PackageName(), msg)
}

func ParseResourceList(v *fncue.CueV) (*ResourceList, error) {
	contents, err := v.Val.MarshalJSON()
	if err != nil {
		return nil, err
	}

	var rl ResourceList

	var list []string
	if json.Unmarshal(contents, &list) == nil {
		rl.Refs = list
		return &rl, nil
	}

	it, err := v.Val.Fields()
	if err != nil {
		return nil, err
	}

	rl.Instances = map[string]*fncue.CueV{}
	for it.Next() {
		rl.Instances[it.Label()] = &fncue.CueV{Val: it.Value()}
	}
	return &rl, nil
}

func (rl *ResourceList) ToPack(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, pkg *pkggraph.Package) (*schema.ResourcePack, error) {
	pack := &schema.ResourcePack{}

	for _, resource := range rl.Refs {
		r, err := parseResourceRef(ctx, pl, pkg.Location, resource)
		if err != nil {
			return nil, err
		}

		pack.ResourceRef = append(pack.ResourceRef, r)
	}

	for name, instance := range rl.Instances {
		instance, err := ParseResourceInstanceFromCue(ctx, env, pl, pkg, name, instance)
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
