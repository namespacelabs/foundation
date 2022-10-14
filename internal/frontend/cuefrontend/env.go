// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"bytes"
	"encoding/json"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema"
)

type EnvMap struct {
	Values map[string]envValue `json:",inline"`
}

type envValue struct {
	v *schema.BinaryConfig_EnvEntry
}

var _ json.Unmarshaler = &envValue{}

func (cem *EnvMap) Parsed() []*schema.BinaryConfig_EnvEntry {
	if cem == nil {
		return nil
	}

	var env []*schema.BinaryConfig_EnvEntry
	for key, value := range cem.Values {
		v := protos.Clone(value.v)
		v.Name = key
		env = append(env, v)
	}

	slices.SortFunc(env, func(a, b *schema.BinaryConfig_EnvEntry) bool {
		return strings.Compare(a.Name, b.Name) < 0
	})

	return env
}

func (ev *envValue) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	tok, err := d.Token()
	if err != nil {
		return err
	}

	if tok == json.Delim('{') {
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			return err
		}

		keys := maps.Keys(m)
		if len(keys) != 1 || keys[0] != "fromSecret" {
			return fnerrors.BadInputError("when setting an object to a env var map, expected a single key `fromSecret`")
		}

		// XXX support setting only the secret name.
		ref, err := schema.ParsePackageRef(m[keys[0]])
		if err != nil {
			return err
		}

		ev.v = &schema.BinaryConfig_EnvEntry{FromSecretRef: ref}
		return nil
	}

	if str, ok := tok.(string); ok {
		ev.v = &schema.BinaryConfig_EnvEntry{Value: str}
		return nil
	}

	return fnerrors.BadInputError("failed to parse value, unexpected token %v", tok)
}
