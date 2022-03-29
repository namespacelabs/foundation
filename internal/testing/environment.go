// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package testing

import (
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/go-ids"
)

func PrepareEnvFrom(env provision.Env) provision.Env {
	slice := devhost.ConfigurationForEnv(env)

	env.Root().DevHost.Configure = slice.WithoutConstraints()

	testInv := ids.NewRandomBase32ID(8)
	testEnv := &schema.Environment{
		Name:    "test-" + testInv,
		Purpose: schema.Environment_TESTING,
		Runtime: "kubernetes",
	}

	return provision.MakeEnv(env.Root(), testEnv)
}