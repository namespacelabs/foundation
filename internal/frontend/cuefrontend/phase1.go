// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/engine/ops"
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

type cueInvokeBinary struct {
	Binary       string                                 `json:"binary"`
	Args         *argsListOrMap                         `json:"args"`
	Mounts       map[string]frontend.InvocationMount    `json:"mount"`
	WorkingDir   string                                 `json:"workingDir"`
	Snapshots    map[string]frontend.InvocationSnapshot `json:"snapshot"`
	NoCache      bool                                   `json:"noCache"`
	RequiresKeys bool                                   `json:"requiresKeys"`
}

func (cib cueInvokeBinary) toFrontend() *frontend.Invocation {
	return &frontend.Invocation{
		Binary:       cib.Binary,
		Args:         cib.Args.Parsed(),
		Mounts:       cib.Mounts,
		WorkingDir:   cib.WorkingDir,
		Snapshots:    cib.Snapshots,
		NoCache:      cib.NoCache,
		RequiresKeys: cib.RequiresKeys,
	}
}

type cueNaming struct {
	DomainName           map[string][]string `json:"domainName"`
	TLSManagedDomainName map[string][]string `json:"tlsManagedDomainName"`
	WithOrg              string              `json:"withOrg"`
}

type cueContainer struct {
	Binary string         `json:"binary"`
	Args   *argsListOrMap `json:"args"`
}

func (p1 phase1plan) EvalProvision(ctx context.Context, env ops.Environment, inputs frontend.ProvisionInputs) (frontend.ProvisionPlan, error) {
	if env.Proto() == nil {
		return frontend.ProvisionPlan{}, fnerrors.InternalError("env is missing .. env")
	}

	vv, left, err := applyInputs(ctx, provisionFuncs(env.Proto(), inputs), p1.Value, p1.Left)
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
		var dec cueInvokeBinary
		if err := with.Val.Decode(&dec); err != nil {
			return pdata, err
		}

		pdata.Provisioning = append(pdata.Provisioning, dec.toFrontend())
	}

	if sidecar := lookupTransition(vv, "sidecar"); sidecar.Exists() {
		var sidecards []cueContainer

		if err := sidecar.Val.Decode(&sidecards); err != nil {
			return pdata, err
		}

		for _, data := range sidecards {
			pdata.Sidecars = append(pdata.Sidecars, frontend.Container{
				Binary: data.Binary,
				Args:   data.Args.Parsed(),
			})
		}
	}

	if init := lookupTransition(vv, "init"); init.Exists() {
		var inits []cueContainer

		if err := init.Val.Decode(&inits); err != nil {
			return pdata, err
		}

		for _, data := range inits {
			pdata.Inits = append(pdata.Inits, frontend.Container{
				Binary: data.Binary,
				Args:   data.Args.Parsed(),
			})
		}
	}

	if naming := lookupTransition(vv, "naming"); naming.Exists() {
		var data cueNaming
		if err := naming.Val.Decode(&data); err != nil {
			return pdata, err
		}

		pdata.Naming = &schema.Naming{
			WithOrg: data.WithOrg,
		}

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

func sortAdditional(a, b *schema.Naming_AdditionalDomainName) bool {
	if a.AllocatedName == b.AllocatedName {
		return strings.Compare(a.Fqdn, b.Fqdn) < 0
	}
	return strings.Compare(a.AllocatedName, b.AllocatedName) < 0
}

type argsListOrMap struct {
	args []string
}

var _ json.Unmarshaler = &argsListOrMap{}

func (args *argsListOrMap) Parsed() []string {
	if args == nil {
		return nil
	}
	return args.args
}

func (args *argsListOrMap) UnmarshalJSON(contents []byte) error {
	var list []string
	if json.Unmarshal(contents, &list) == nil {
		args.args = list
		return nil
	}

	var m map[string]string
	if json.Unmarshal(contents, &m) == nil {
		for k, v := range m {
			if v != "" {
				args.args = append(args.args, fmt.Sprintf("--%s=%s", k, v))
			} else {
				args.args = append(args.args, fmt.Sprintf("--%s", k))
			}
		}
		return nil
	}

	return fnerrors.InternalError("args: expected a list of strings, or a map of string to string")
}

func lookupTransition(vv *fncue.CueV, name string) *fncue.CueV {
	new := vv.LookupPath("configure." + name)
	if new.Exists() {
		return new
	}

	return vv.LookupPath("extend." + name)
}

func provisionFuncs(env *schema.Environment, inputs frontend.ProvisionInputs) *EvalFuncs {
	return newFuncs().
		WithFetcher(fncue.WorkspaceIKw, FetchServerWorkspace(inputs.Workspace, inputs.ServerLocation)).
		WithFetcher(fncue.EnvIKw, FetchEnv(env, inputs.Workspace))
}
