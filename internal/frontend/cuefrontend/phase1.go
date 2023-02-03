// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"
	"strings"

	"cuelang.org/go/cue"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/args"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/binary"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type phase1plan struct {
	owner   schema.PackageName
	partial *fncue.Partial

	Value *fncue.CueV
	Left  []fncue.KeyAndPath // injected values left to be filled.
}

type cueWithPackageName struct {
	PackageName string `json:"packageName"`
}

type cueStack struct {
	Append []cueWithPackageName `json:"append"`
}

type cueContainer struct {
	Name   string              `json:"name"`
	Binary string              `json:"binary"`
	Args   *args.ArgsListOrMap `json:"args"`
}

type evalProvisionResult struct {
	pkggraph.ProvisionPlan
	DeclaredStack  []schema.PackageName
	Sidecars       []*schema.Container
	InitContainers []*schema.Container
}

func (p1 phase1plan) EvalProvision(ctx context.Context, env *schema.Environment, inputs pkggraph.ProvisionInputs) (*evalProvisionResult, error) {
	vv, left, err := fncue.SerializedEval3(p1.partial, func() (*fncue.CueV, []fncue.KeyAndPath, error) {
		return applyInputs(ctx, provisionFuncs(env, inputs), p1.Value, p1.Left)
	})
	if err != nil {
		return nil, err
	}

	var pdata evalProvisionResult

	pdata.Startup = phase2plan{owner: p1.owner, partial: p1.partial, Value: vv, Left: left}

	if stackVal := lookupTransition(vv, "stack"); stackVal.Exists() {
		var stack cueStack
		if err := stackVal.Val.Decode(&stack); err != nil {
			return nil, err
		}

		var packages schema.PackageList
		for _, p := range stack.Append {
			packages.Add(schema.PackageName(p.PackageName))
		}

		pdata.DeclaredStack = packages.PackageNames()
	}

	// Not using parseBinaryInvocation because it may modify the Package which is not allowed in phase1.
	// This parsing needs to happen in phase1 for "std/secrets" where "$workspace" value is used.
	if with := vv.LookupPath("configure.with"); with.Exists() {
		binName, err := with.LookupPath("binary").Val.String()
		if err != nil {
			return nil, err
		}
		binRef, err := schema.ParsePackageRef(p1.owner, binName)
		if err != nil {
			return nil, err
		}

		inv, err := binary.ParseBinaryInvocationForBinaryRef(ctx, p1.owner, binRef, with)
		if err != nil {
			return nil, err
		}
		pdata.ComputePlanWith = append(pdata.ComputePlanWith, inv)
	}

	if sidecar := lookupTransition(vv, "sidecar"); sidecar.Exists() {
		pdata.Sidecars, err = parseContainers(p1.owner, "sidecar", sidecar.Val)
		if err != nil {
			return nil, err
		}
	}

	if init := lookupTransition(vv, "init"); init.Exists() {
		pdata.InitContainers, err = parseContainers(p1.owner, "init", init.Val)
		if err != nil {
			return nil, err
		}
	}

	if naming := lookupTransition(vv, "naming"); naming.Exists() {
		pdata.Naming, err = ParseNaming(naming)
		if err != nil {
			return nil, err
		}
	}

	return &pdata, nil
}

func parseContainers(owner schema.PackageName, kind string, v cue.Value) ([]*schema.Container, error) {
	// XXX remove ListKind version.
	if v.Kind() == cue.ListKind {
		var containers []cueContainer

		if err := v.Decode(&containers); err != nil {
			return nil, err
		}

		var parsed []*schema.Container
		for k, data := range containers {
			binRef, err := schema.ParsePackageRef(owner, data.Binary)
			if err != nil {
				return nil, err
			}

			if data.Name == "" {
				return nil, fnerrors.New("%s #%d: name is required", kind, k)
			}

			parsed = append(parsed, &schema.Container{
				Owner:     schema.MakePackageSingleRef(owner),
				Name:      data.Name,
				BinaryRef: binRef,
				Args:      data.Args.Parsed(),
			})
		}

		return parsed, nil
	}

	var containers map[string]cueContainer
	if err := v.Decode(&containers); err != nil {
		return nil, err
	}

	var parsed []*schema.Container
	for name, data := range containers {
		if data.Name != "" && data.Name != name {
			return nil, fnerrors.New("%s: inconsistent container name %q", name, data.Name)
		}

		binRef, err := schema.ParsePackageRef(owner, data.Binary)
		if err != nil {
			return nil, err
		}

		parsed = append(parsed, &schema.Container{
			Owner:     schema.MakePackageSingleRef(owner),
			Name:      name,
			BinaryRef: binRef,
			Args:      data.Args.Parsed(),
		})
	}

	return parsed, nil
}

func sortAdditional(a, b *schema.Naming_AdditionalDomainName) bool {
	if a.AllocatedName == b.AllocatedName {
		return strings.Compare(a.Fqdn, b.Fqdn) < 0
	}
	return strings.Compare(a.AllocatedName, b.AllocatedName) < 0
}

func lookupTransition(vv *fncue.CueV, name string) *fncue.CueV {
	new := vv.LookupPath("configure." + name)
	if new.Exists() {
		return new
	}

	return vv.LookupPath("extend." + name)
}

func provisionFuncs(env *schema.Environment, inputs pkggraph.ProvisionInputs) *EvalFuncs {
	return newFuncs().
		WithFetcher(fncue.WorkspaceIKw, FetchServerWorkspace(inputs.ServerLocation))
}
