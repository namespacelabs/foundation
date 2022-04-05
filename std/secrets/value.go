// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package secrets

import (
	"fmt"
	"io/ioutil"
)

func (v *Value) Value() ([]byte, error) {
	return ioutil.ReadFile(v.Path)
}

func (v *Value) MustValue() []byte {
	contents, err := v.Value()
	if err != nil {
		panic(fmt.Sprintf("failed to load required secret: %q: %v", v.Name, err))
	}
	return contents
}
