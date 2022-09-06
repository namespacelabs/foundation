// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package service

import (
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/providers/aws"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
)

type env struct {
	config    planning.Configuration
	workspace planning.Workspace
	env       *schema.Environment
}

func makeEnv(plan *schema.DeployPlan, awsConf *aws.Conf) *env {
	messages := []proto.Message{&client.HostEnv{Incluster: true}}

	if awsConf != nil {
		messages = append(messages, awsConf)
	}

	return &env{
		config: planning.MakeConfigurationWith(plan.Environment.Name, planning.ConfigurationSlice{
			Configuration: protos.WrapAnysOrDie(messages...),
		}),
		workspace: planning.MakeWorkspace(plan.Workspace, nil),
		env:       plan.Environment,
	}
}

func (e env) Configuration() planning.Configuration { return e.config }
func (e env) ErrorLocation() string                 { return e.workspace.ModuleName() }
func (e env) Workspace() planning.Workspace         { return e.workspace }
func (e env) Environment() *schema.Environment      { return e.env }
