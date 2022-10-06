// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution"
)

func Serialize(ws *schema.Workspace, env *schema.Environment, stack *schema.Stack, computed *Plan, focus []string) *schema.DeployPlan {
	deployPlan := &schema.DeployPlan{
		Workspace: &schema.Workspace{
			ModuleName: ws.ModuleName,
			Dep:        ws.Dep,
			Replace:    ws.Replace,
		},
		Environment:     env,
		Stack:           stack,
		IngressFragment: computed.IngressFragments,
		Program:         execution.Serialize(computed.Deployer),
		Hints:           computed.Hints,
		FocusServer:     focus,
	}

	return deployPlan
}
