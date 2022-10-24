// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package binary

import (
	"context"

	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/args"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type cueInvocationSnapshot struct {
	FromWorkspace string `json:"fromWorkspace"`
	Optional      bool   `json:"optional"`
	RequireFile   bool   `json:"requireFile"`
}

type cueInvokeBinary struct {
	Args         *args.ArgsListOrMap              `json:"args"`
	WorkingDir   string                           `json:"workingDir"`
	Snapshots    map[string]cueInvocationSnapshot `json:"snapshot"`
	NoCache      bool                             `json:"noCache"`
	RequiresKeys bool                             `json:"requiresKeys"`
	Inject       []string                         `json:"inject"`
}

func ParseBinaryInvocationField(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, pkg *pkggraph.Package, binaryName string, cuePath string, v *fncue.CueV) (*schema.Invocation, error) {
	if b := v.LookupPath(cuePath); b.Exists() {
		return parseBinaryInvocation(ctx, env, pl, pkg, binaryName, b)
	}

	return nil, nil
}

func parseBinaryInvocation(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, pkg *pkggraph.Package, binaryName string, v *fncue.CueV) (*schema.Invocation, error) {
	var cib cueInvokeBinary
	if err := v.Val.Decode(&cib); err != nil {
		return nil, err
	}

	binRef, err := ParseImage(ctx, env, pl, pkg, binaryName, v, ParseImageOpts{Required: true})
	if err != nil {
		return nil, err
	}

	return ParseBinaryInvocationForBinaryRef(ctx, binRef, v)
}

func ParseBinaryInvocationForBinaryRef(ctx context.Context, binRef *schema.PackageRef, v *fncue.CueV) (*schema.Invocation, error) {
	var cib cueInvokeBinary
	if err := v.Val.Decode(&cib); err != nil {
		return nil, err
	}

	inv := &schema.Invocation{
		BinaryRef:    binRef,
		Args:         cib.Args.Parsed(),
		WorkingDir:   cib.WorkingDir,
		NoCache:      cib.NoCache,
		RequiresKeys: cib.RequiresKeys,
	}

	for _, inject := range cib.Inject {
		inv.Inject = append(inv.Inject, &schema.Invocation_ValueInjection{
			Type: inject,
		})
	}

	for k, v := range cib.Snapshots {
		if inv.Snapshots == nil {
			inv.Snapshots = map[string]*schema.InvocationSnapshot{}
		}
		inv.Snapshots[k] = &schema.InvocationSnapshot{
			FromWorkspace: v.FromWorkspace,
			Optional:      v.Optional,
			RequireFile:   v.RequireFile,
		}
	}

	return inv, nil
}
