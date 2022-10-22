// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package schema

type DeployableClass string

const (
	// Represents a horizontally scalable stateless deployment.
	DeployableClass_STATELESS DeployableClass = "deployableclass.namespace.so/stateless"
	// Represents a stateful deployment.
	DeployableClass_STATEFUL DeployableClass = "deployableclass.namespace.so/stateful"
	// Represents a one-shot run.
	DeployableClass_ONESHOT DeployableClass = "deployableclass.namespace.so/one-shot"
	// Represents an internal one-shot run whose orchestration management is manual.
	DeployableClass_MANUAL DeployableClass = "deployableclass.namespace.so/manual"
)
