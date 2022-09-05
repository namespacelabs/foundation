// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package service

import (
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/providers/aws"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
)

type env struct {
	config    planning.Configuration
	workspace *schema.Workspace
	env       *schema.Environment
}

func makeEnv(plan *schema.DeployPlan, awsConf *aws.Conf) *env {
	var config []*anypb.Any
	config = append(config, protos.WrapAnysOrDie(&client.HostEnv{Incluster: true})...)

	if awsConf != nil {
		config = append(config, protos.WrapAnysOrDie(awsConf)...)
	}

	return &env{
		config:    planning.MakeConfigurationWith(plan.Environment.Name, config, nil),
		workspace: plan.Workspace,
		env:       plan.Environment,
	}
}

func (e env) Configuration() planning.Configuration             { return e.config }
func (e env) ErrorLocation() string                             { return e.workspace.ModuleName }
func (e env) Workspace() *schema.Workspace                      { return e.workspace }
func (e env) WorkspaceLoadedFrom() *schema.Workspace_LoadedFrom { return nil } // Not needed in orchestrator
func (e env) Environment() *schema.Environment                  { return e.env }
