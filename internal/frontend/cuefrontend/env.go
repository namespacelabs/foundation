// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"encoding/json"
	"strings"

	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/schema"
)

type EnvMap struct {
	values map[string]string
}

var _ json.Unmarshaler = &EnvMap{}

func (cem *EnvMap) UnmarshalJSON(data []byte) error {
	var m map[string]string

	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	cem.values = m
	return nil
}

func (cem *EnvMap) Parsed() []*schema.BinaryConfig_EnvEntry {
	if cem == nil {
		return nil
	}

	var env []*schema.BinaryConfig_EnvEntry
	for key, value := range cem.values {
		env = append(env, &schema.BinaryConfig_EnvEntry{
			Name:  key,
			Value: value,
		})
	}

	slices.SortFunc(env, func(a, b *schema.BinaryConfig_EnvEntry) bool {
		return strings.Compare(a.Name, b.Name) < 0
	})

	return env
}
