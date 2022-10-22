// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package secrets

import (
	"fmt"
	"os"
)

func (v *Value) Value() ([]byte, error) {
	return os.ReadFile(v.Path)
}

func (v *Value) MustValue() []byte {
	contents, err := v.Value()
	if err != nil {
		panic(fmt.Sprintf("failed to load required secret: %q: %v", v.Name, err))
	}
	return contents
}
