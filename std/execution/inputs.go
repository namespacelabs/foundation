// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package execution

import "google.golang.org/protobuf/proto"

type Input struct {
	Message  proto.Message
	Instance any
}

type Inputs map[string]Input

var InputsInjection = Define[Inputs]("namespace.ops.inputs")
