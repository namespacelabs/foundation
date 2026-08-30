// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// This package is separate so we can use generics, schema is stricly go 1.17.
package schemahelper

import (
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

type HasGetServiceMetadata interface {
	GetServiceMetadata() []*schema.ServiceMetadata
}

func CombineServiceMetadata[V HasGetServiceMetadata](list []V) []*schema.ServiceMetadata {
	var combined []*schema.ServiceMetadata
	for _, m := range list {
		combined = append(combined, m.GetServiceMetadata()...)
	}
	return combined
}

func UnmarshalServiceMetadata[V proto.Message](mds []*schema.ServiceMetadata, kind string) (V, error) {
	var empty V

	for _, md := range mds {
		if md.Kind == kind {
			if md.Details == nil {
				break
			}

			msg, err := md.Details.UnmarshalNew()
			if err != nil {
				return empty, err
			}

			v, ok := msg.(V)
			if !ok {
				return empty, fnerrors.InternalError("unexpected type %q", md.Details.TypeUrl)
			}

			return v, nil
		}
	}

	return empty, nil
}
