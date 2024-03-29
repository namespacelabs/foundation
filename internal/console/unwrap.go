// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package console

import "namespacelabs.dev/foundation/std/tasks"

func UnwrapSink(sink tasks.ActionSink) tasks.ActionSink {
	if sink != nil {
		switch x := sink.(type) {
		case hasUnwrap:
			return UnwrapSink(x.Unwrap())
		default:
			return x
		}
	}

	return nil
}

type hasUnwrap interface {
	Unwrap() tasks.ActionSink
}
