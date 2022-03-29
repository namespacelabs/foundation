// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
)

type phase1plan struct {
	Value *fncue.CueV
	Left  []fncue.KeyAndPath // injected values left to be filled.
}

type cueWithPackageName struct {
	PackageName string `json:"packageName"`
}

type cueStack struct {
	Append []cueWithPackageName `json:"append"`
}

type cueProvisioningConf struct {
	With *frontend.Invocation `json:"with"`
}

type cueNaming struct {
	DomainName           string `json:"domainName"`
	TLSManagedDomainName string `json:"tlsManagedDomainName"`
	WithOrg              string `json:"withOrg"`
}

func (p1 phase1plan) EvalProvision(ctx context.Context, inputs frontend.ProvisionInputs) (frontend.ProvisionPlan, error) {
	vv, left, err := applyInputs(ctx, provisionFuncs(inputs), p1.Value, p1.Left)
	if err != nil {
		return frontend.ProvisionPlan{}, err
	}

	var pdata frontend.ProvisionPlan

	pdata.Startup = phase2plan{Value: vv, Left: left}

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
		dec := &frontend.Invocation{}
		if err := with.Val.Decode(dec); err != nil {
			return pdata, err
		}
		pdata.Provisioning = dec
	} else if provisioning := vv.LookupPath("extend.provisioning"); provisioning.Exists() {
		var dec cueProvisioningConf
		if err := provisioning.Val.Decode(&dec); err != nil {
			return pdata, err
		}
		if dec.With == nil {
			return pdata, fnerrors.UserError(nil, "provisioning.with can't be empty")
		}
		pdata.Provisioning = dec.With
	}

	if init := lookupTransition(vv, "init"); init.Exists() {
		if err := init.Val.Decode(&pdata.Inits); err != nil {
			return pdata, err
		}
	}

	if naming := lookupTransition(vv, "naming"); naming.Exists() {
		var data cueNaming
		if err := naming.Val.Decode(&data); err != nil {
			return pdata, err
		}

		if data.DomainName != "" || data.WithOrg != "" {
			pdata.Naming = &schema.Naming{
				DomainName:           data.DomainName,
				TlsManagedDomainName: data.TLSManagedDomainName,
				WithOrg:              data.WithOrg,
			}
		}
	}

	return pdata, nil
}

func lookupTransition(vv *fncue.CueV, name string) *fncue.CueV {
	new := vv.LookupPath("configure." + name)
	if new.Exists() {
		return new
	}

	return vv.LookupPath("extend." + name)
}

func provisionFuncs(inputs frontend.ProvisionInputs) *EvalFuncs {
	return newFuncs().
		WithFetcher(fncue.WorkspaceIKw, FetchServerWorkspace(inputs.Workspace, inputs.ServerLocation)).
		WithFetcher(fncue.EnvIKw, FetchEnv(inputs.Env))
}