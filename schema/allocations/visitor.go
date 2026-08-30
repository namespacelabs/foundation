// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package allocations

import (
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/codegen/protos/fnany"
	"namespacelabs.dev/foundation/schema"
)

func Visit[V proto.Message](allocs []*schema.Allocation, owner schema.PackageName, tmpl V, f func(*schema.Allocation_Instance, *schema.Instantiate, V) error) error {
	typeURL := fnany.TypeURL(owner, tmpl)

	for _, alloc := range allocs {
		for _, instance := range alloc.Instance {
			for _, i := range instance.Instantiated {
				if i.GetConstructor().GetTypeUrl() == typeURL {
					copy := proto.Clone(tmpl)
					if err := proto.Unmarshal(i.Constructor.GetValue(), copy); err != nil {
						return err
					}

					if err := f(instance, i, copy.(V)); err != nil {
						return err
					}
				}
			}

			if err := Visit(instance.DownstreamAllocation, owner, tmpl, f); err != nil {
				return err
			}
		}
	}

	return nil
}
