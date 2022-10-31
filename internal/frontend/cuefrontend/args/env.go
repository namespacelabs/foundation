// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package args

import (
	"bytes"
	"encoding/json"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

type EnvMap struct {
	Values map[string]envValue
}

type envValue struct {
	value               string
	fromSecret          string
	withServiceEndpoint *envServiceEndpoint
}

type envServiceEndpoint struct {
	ServerRef   string `json:"serverRef"`
	ServiceName string `json:"service"`
}

var _ json.Unmarshaler = &EnvMap{}
var _ json.Unmarshaler = &envValue{}

func (cem *EnvMap) UnmarshalJSON(data []byte) error {
	var values map[string]envValue
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	cem.Values = values
	return nil
}

func (cem *EnvMap) Parsed(owner schema.PackageName) ([]*schema.BinaryConfig_EnvEntry, error) {
	if cem == nil {
		return nil, nil
	}

	var env []*schema.BinaryConfig_EnvEntry
	for key, value := range cem.Values {
		out := &schema.BinaryConfig_EnvEntry{
			Name: key,
		}
		if value.value != "" {
			out.Value = value.value
		} else if value.fromSecret != "" {
			secretRef, err := schema.ParsePackageRef(owner, value.fromSecret)
			if err != nil {
				return nil, err
			}
			out.FromSecretRef = secretRef
		} else if value.withServiceEndpoint != nil {
			serverRef, err := schema.ParsePackageRef(owner, value.withServiceEndpoint.ServerRef)
			if err != nil {
				return nil, err
			}
			out.WithServiceEndpoint = &schema.ServiceRef{ServerRef: serverRef, ServiceName: value.withServiceEndpoint.ServiceName}
		}
		env = append(env, out)
	}

	slices.SortFunc(env, func(a, b *schema.BinaryConfig_EnvEntry) bool {
		return strings.Compare(a.Name, b.Name) < 0
	})

	return env, nil
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

		ev.fromSecret = m[keys[0]]
		return nil
	}

	if str, ok := tok.(string); ok {
		ev.value = str
		return nil
	}

	return fnerrors.BadInputError("failed to parse value, unexpected token %v", tok)
}
