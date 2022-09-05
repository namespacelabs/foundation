// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package orchestration

import (
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
)

type orchEnv struct {
	ctx planning.Context
}

func (e orchEnv) ErrorLocation() string                 { return e.ctx.ErrorLocation() }
func (e orchEnv) Workspace() planning.Workspace         { return e.ctx.Workspace() }
func (e orchEnv) Configuration() planning.Configuration { return e.ctx.Configuration() }

// We use a static environment here, since the orchestrator has global scope.
// TODO remodel.
func (e orchEnv) Environment() *schema.Environment {
	return &schema.Environment{
		Name:      "fn-admin",
		Runtime:   e.ctx.Environment().Runtime,
		Ephemeral: false,
		Purpose:   schema.Environment_PRODUCTION, // TODO - this can't be empty, since std/runtime/kubernetes/extension.cue checks it.
	}
}
