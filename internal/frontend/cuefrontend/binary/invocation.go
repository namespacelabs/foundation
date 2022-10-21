// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package binary

import (
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/args"
	"namespacelabs.dev/foundation/schema"
)

type cueInvocationSnapshot struct {
	FromWorkspace string `json:"fromWorkspace"`
	Optional      bool   `json:"optional"`
	RequireFile   bool   `json:"requireFile"`
}

type CueInvokeBinary struct {
	Binary       string                           `json:"binary"`
	Args         *args.ArgsListOrMap              `json:"args"`
	WorkingDir   string                           `json:"workingDir"`
	Snapshots    map[string]cueInvocationSnapshot `json:"snapshot"`
	NoCache      bool                             `json:"noCache"`
	RequiresKeys bool                             `json:"requiresKeys"`
	Inject       []string                         `json:"inject"`
}

func (cib *CueInvokeBinary) ToInvocation(owner schema.PackageName) (*schema.Invocation, error) {
	if cib == nil {
		return nil, nil
	}

	binRef, err := schema.ParsePackageRef(owner, cib.Binary)
	if err != nil {
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
