// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package ops

import "google.golang.org/protobuf/proto"

type Input struct {
	Message proto.Message
}

type Inputs map[string]Input

var InputsInjection = Define[Inputs]("namespace.ops.inputs")
