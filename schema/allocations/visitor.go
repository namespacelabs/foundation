// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package allocations

import (
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/source/protos/fnany"
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
