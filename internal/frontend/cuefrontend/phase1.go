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
	"namespacelabs.dev/foundation/std/cfg"
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

func (p1 phase1plan) EvalProvision(ctx context.Context, env cfg.Context, inputs pkggraph.ProvisionInputs) (pkggraph.ProvisionPlan, error) {
	if env.Environment() == nil {
		return pkggraph.ProvisionPlan{}, fnerrors.InternalError("env is missing .. env")
	}

	vv, left, err := fncue.SerializedEval3(p1.partial, func() (*fncue.CueV, []fncue.KeyAndPath, error) {
		return applyInputs(ctx, provisionFuncs(env.Environment(), inputs), p1.Value, p1.Left)
	})
	if err != nil {
		return pkggraph.ProvisionPlan{}, err
	}

	var pdata pkggraph.ProvisionPlan

	pdata.Startup = phase2plan{owner: p1.owner, partial: p1.partial, Value: vv, Left: left}

	if stackVal := lookupTransition(vv, "stack"); stackVal.Exists() {
		var stack cueStack
		if err := stackVal.Val.Decode(&stack); err != nil {
			return pdata, err
		}

		for _, p := range stack.Append {
			pdata.DeclaredStack = append(pdata.DeclaredStack, schema.PackageName(p.PackageName))
		}
	}

	// Not using parseBinaryInvocation because it may modify the Package which is not allowed in phase1.
	// This parsing needs to happen in phase1 for "std/secrets" where "$workspace" value is used.
	if with := vv.LookupPath("configure.with"); with.Exists() {
		binName, err := with.LookupPath("binary").Val.String()
		if err != nil {
			return pdata, err
		}
		binRef, err := schema.ParsePackageRef(p1.owner, binName)
		if err != nil {
			return pdata, err
		}

		inv, err := binary.ParseBinaryInvocationForBinaryRef(ctx, p1.owner, binRef, with)
		if err != nil {
			return pdata, err
		}
		pdata.ComputePlanWith = append(pdata.ComputePlanWith, inv)
	}

	if sidecar := lookupTransition(vv, "sidecar"); sidecar.Exists() {
		pdata.Sidecars, err = parseContainers(p1.owner, "sidecar", sidecar.Val)
		if err != nil {
			return pdata, err
		}
	}

	if init := lookupTransition(vv, "init"); init.Exists() {
		pdata.Inits, err = parseContainers(p1.owner, "init", init.Val)
		if err != nil {
			return pdata, err
		}
	}

	if naming := lookupTransition(vv, "naming"); naming.Exists() {
		pdata.Naming, err = ParseNaming(naming)
		if err != nil {
			return pdata, err
		}
	}

	return pdata, nil
}

func parseContainers(owner schema.PackageName, kind string, v cue.Value) ([]*schema.SidecarContainer, error) {
	// XXX remove ListKind version.
	if v.Kind() == cue.ListKind {
		var containers []cueContainer

		if err := v.Decode(&containers); err != nil {
			return nil, err
		}

		var parsed []*schema.SidecarContainer
		for k, data := range containers {
			binRef, err := schema.ParsePackageRef(owner, data.Binary)
			if err != nil {
				return nil, err
			}

			if data.Name == "" {
				return nil, fnerrors.New("%s #%d: name is required", kind, k)
			}

			parsed = append(parsed, &schema.SidecarContainer{
				Owner:     owner.String(),
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

	var parsed []*schema.SidecarContainer
	for name, data := range containers {
		if data.Name != "" && data.Name != name {
			return nil, fnerrors.New("%s: inconsistent container name %q", name, data.Name)
		}

		binRef, err := schema.ParsePackageRef(owner, data.Binary)
		if err != nil {
			return nil, err
		}

		parsed = append(parsed, &schema.SidecarContainer{
			Owner:     owner.String(),
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
