// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package files

import (
	"encoding/json"
	"os"
)

func WriteJson(path string, m any, perm os.FileMode) error {
	serialized, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, serialized, perm); err != nil {
		return err
	}

	return nil
}
