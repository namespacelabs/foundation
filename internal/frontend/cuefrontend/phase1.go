// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"context"
	"strings"

	"cuelang.org/go/cue"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
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

type cueNaming struct {
	DomainName           map[string][]string `json:"domainName"`
	TLSManagedDomainName map[string][]string `json:"tlsManagedDomainName"`
	WithOrg              string              `json:"withOrg"`
}

type cueContainer struct {
	Name   string         `json:"name"`
	Binary string         `json:"binary"`
	Args   *ArgsListOrMap `json:"args"`
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

	if with := vv.LookupPath("configure.with"); with.Exists() {
		var dec CueInvokeBinary
		if err := with.Val.Decode(&dec); err != nil {
			return pdata, err
		}

		inv, err := dec.ToInvocation(p1.owner)
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

	pdata.Naming = &schema.Naming{
		EnableNamespaceManaged: true,
	}

	if naming := lookupTransition(vv, "naming"); naming.Exists() {
		var data cueNaming
		if err := naming.Val.Decode(&data); err != nil {
			return pdata, err
		}

		pdata.Naming.WithOrg = data.WithOrg

		for k, v := range data.DomainName {
			for _, fqdn := range v {
				pdata.Naming.AdditionalUserSpecified = append(pdata.Naming.AdditionalUserSpecified, &schema.Naming_AdditionalDomainName{
					AllocatedName: k,
					Fqdn:          fqdn,
				})
			}
		}

		for k, v := range data.TLSManagedDomainName {
			for _, fqdn := range v {
				pdata.Naming.AdditionalTlsManaged = append(pdata.Naming.AdditionalTlsManaged, &schema.Naming_AdditionalDomainName{
					AllocatedName: k,
					Fqdn:          fqdn,
				})
			}
		}

		slices.SortFunc(pdata.Naming.AdditionalUserSpecified, sortAdditional)
		slices.SortFunc(pdata.Naming.AdditionalTlsManaged, sortAdditional)
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
				return nil, fnerrors.UserError(nil, "%s #%d: name is required", kind, k)
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
			return nil, fnerrors.UserError(nil, "%s: inconsistent container name %q", name, data.Name)
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
		WithFetcher(fncue.WorkspaceIKw, FetchServerWorkspace(inputs.ServerLocation)).
		WithFetcher(fncue.EnvIKw, FetchEnv(env))
}
