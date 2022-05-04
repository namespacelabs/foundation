// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package configure

import (
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/source/protos/fnany"
)

func VisitAllocs[V proto.Message](allocs []*schema.Allocation, pkg schema.PackageName, tmpl V, f func(*schema.Allocation_Instance, *schema.Instantiate, V) error) error {
	typeURL := fnany.TypeURL(pkg, tmpl)

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

			if err := VisitAllocs(instance.DownstreamAllocation, pkg, tmpl, f); err != nil {
				return err
			}
		}
	}

	return nil
}
