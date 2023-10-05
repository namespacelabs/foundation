// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package grpc

import (
	"namespacelabs.dev/foundation/std/go/core"
)

var DrainFunc func()

func SetDrainFunc(f func()) {
	core.AssertNotRunning("grpc.SetDrainFunc")

	if DrainFunc != nil {
		panic("drain func was already set")
	}

	DrainFunc = f
}
