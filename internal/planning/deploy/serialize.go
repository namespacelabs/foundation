// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deploy

import (
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution"
)

func Serialize(ws *schema.Workspace, env *schema.Environment, stack *schema.Stack, computed *Plan, focus schema.PackageList) *schema.DeployPlan {
	deployPlan := &schema.DeployPlan{
		Workspace: &schema.Workspace{
			ModuleName: ws.ModuleName,
			Dep:        ws.Dep,
			Replace:    ws.Replace,
		},
		Environment:        env,
		Stack:              stack,
		IngressFragment:    computed.IngressFragments,
		Program:            execution.Serialize(computed.Deployer),
		FocusServer:        focus.PackageNamesAsString(),
		NamespaceReference: computed.NamespaceReference,
	}

	return deployPlan
}
