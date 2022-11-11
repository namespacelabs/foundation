// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package support

import (
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

func MergeEnvs(base []*schema.BinaryConfig_EnvEntry, additional []*schema.BinaryConfig_EnvEntry) ([]*schema.BinaryConfig_EnvEntry, error) {
	merged := append(base, additional...)

	// Check for collisions.
	index := make(map[string]*schema.BinaryConfig_EnvEntry)
	for _, entry := range merged {
		if existing, ok := index[entry.Name]; ok {
			if proto.Equal(entry, existing) {
				continue
			}
			return nil, fnerrors.BadInputError("incompatible values being set for env key %q (%v vs %v)", entry.Name, entry, existing)
		} else {
			index[entry.Name] = entry
		}
	}

	return merged, nil
}
