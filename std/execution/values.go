// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package execution

import (
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/schema"
)

func ComputedValue[M proto.Message](inv *schema.SerializedInvocation, name string) (M, error) {
	var empty M

	for _, x := range inv.Computed {
		if x.Name == name {
			v, err := x.Value.UnmarshalNew()
			if err != nil {
				return empty, err
			}
			return v.(M), nil
		}
	}

	return empty, nil
}
