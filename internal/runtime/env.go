// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package runtime

import (
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

func SetEnv(merged []*schema.BinaryConfig_EnvEntry, entry *schema.BinaryConfig_EnvEntry) ([]*schema.BinaryConfig_EnvEntry, error) {
	for _, existing := range merged {
		if entry.Name == existing.Name {
			if proto.Equal(entry, existing) {
				continue
			}

			return nil, fnerrors.BadInputError("incompatible values being set for env key %q (%v vs %v)", entry.Name, entry, existing)
		}
	}

	return append(merged, entry), nil
}
