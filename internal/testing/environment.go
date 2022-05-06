// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package testing

import (
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/go-ids"
)

type factory struct {
	root      *workspace.Root
	ephemeral bool
}

func EnvFactory(env provision.Env, ephemeral bool) factory {
	slice := devhost.ConfigurationForEnv(env)

	env.Root().DevHost.Configure = slice.WithoutConstraints()

	return factory{
		root:      env.Root(),
		ephemeral: ephemeral,
	}

}

func (f factory) PrepareTestEnv() provision.Env {
	testInv := ids.NewRandomBase32ID(8)
	testEnv := &schema.Environment{
		Name:      "test-" + testInv,
		Purpose:   schema.Environment_TESTING,
		Runtime:   "kubernetes",
		Ephemeral: f.ephemeral,
	}

	return provision.MakeEnv(f.root, testEnv)
}

func (f factory) PrepareControllerEnv() provision.Env {
	testEnv := &schema.Environment{
		Name:      "test-controller",
		Purpose:   schema.Environment_TESTING,
		Runtime:   "kubernetes",
		Ephemeral: false,
	}

	return provision.MakeEnv(f.root, testEnv)
}
