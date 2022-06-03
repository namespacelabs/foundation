// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devhost

import (
	"encoding/json"
	"fmt"

	"namespacelabs.dev/foundation/schema"
)

type Selector interface {
	Description() string
	HashKey() string
	Select(*schema.DevHost) ConfSlice
}

func Select(devHost *schema.DevHost, selector Selector) ConfSlice {
	return selector.Select(devHost)
}

func ByEnvironment(env *schema.Environment) Selector {
	return byEnv{env}
}

type byEnv struct{ env *schema.Environment }

func (e byEnv) Description() string { return e.env.Name }

func (e byEnv) HashKey() string {
	serialized, err := json.Marshal(e.env)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("env:%s", serialized)
}

func (e byEnv) Select(devHost *schema.DevHost) ConfSlice {
	var slice ConfSlice

	for _, cfg := range devHost.GetConfigure() {
		if cfg.Purpose != 0 && cfg.Purpose != e.env.GetPurpose() {
			continue
		}
		if cfg.Runtime != "" && cfg.Runtime != e.env.GetRuntime() {
			continue
		}
		if cfg.Name != "" && cfg.Name != e.env.GetName() {
			continue
		}

		slice.merged = append(slice.merged, cfg)
	}

	return slice
}
