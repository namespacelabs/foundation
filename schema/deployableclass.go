// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package schema

type DeployableClass string

const (
	// Represents a horizontally scalable stateless deployment.
	DeployableClass_STATELESS DeployableClass = "deployableclass.namespace.so/stateless"
	// Represents a stateful deployment.
	DeployableClass_STATEFUL DeployableClass = "deployableclass.namespace.so/stateful"
	// Represents a one-shot run.
	DeployableClass_ONESHOT DeployableClass = "deployableclass.namespace.so/one-shot"
)
