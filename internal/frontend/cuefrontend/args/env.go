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
	fromServiceEndpoint string
	fromResourceField   *resourceFieldSyntax
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
		} else if value.fromServiceEndpoint != "" {
			serviceRef, err := schema.ParsePackageRef(owner, value.fromServiceEndpoint)
			if err != nil {
				return nil, err
			}
			out.FromServiceEndpoint = &schema.ServiceRef{
				ServerRef:   &schema.PackageRef{PackageName: serviceRef.PackageName},
				ServiceName: serviceRef.Name,
			}
		} else if value.fromResourceField != nil {
			resourceRef, err := schema.ParsePackageRef(owner, value.fromResourceField.Resource)
			if err != nil {
				return nil, err
			}

			out.FromResourceField = &schema.ResourceConfigFieldSelector{
				Resource:      resourceRef,
				FieldSelector: value.fromResourceField.FieldRef,
			}
		}
		env = append(env, out)
	}

	slices.SortFunc(env, func(a, b *schema.BinaryConfig_EnvEntry) bool {
		return strings.Compare(a.Name, b.Name) < 0
	})

	return env, nil
}

var validKeys = []string{"fromSecret", "fromServiceEndpoint", "fromResourceField"}

func (ev *envValue) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	tok, err := d.Token()
	if err != nil {
		return err
	}

	if tok == json.Delim('{') {
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			return err
		}

		keys := maps.Keys(m)
		if len(keys) != 1 || !slices.Contains(validKeys, keys[0]) {
			return fnerrors.BadInputError("when setting an object to a env var map, expected a single key, one of %s", strings.Join(validKeys, ", "))
		}

		var err error
		switch keys[0] {
		case "fromSecret":
			ev.fromSecret, err = mustString("fromSecret", m[keys[0]])

		case "fromServiceEndpoint":
			ev.fromServiceEndpoint, err = mustString("fromServiceEndpoint", m[keys[0]])

		case "fromResourceField":
			var ref resourceFieldSyntax
			if err := reUnmarshal(m[keys[0]], &ref); err != nil {
				return fnerrors.BadInputError("failed to parse fromResourceField: %w", err)
			}

			ev.fromResourceField = &ref
			return nil
		}

		return err
	}

	if str, ok := tok.(string); ok {
		ev.value = str
		return nil
	}

	return fnerrors.BadInputError("failed to parse value, unexpected token %v", tok)
}

func mustString(what string, value any) (string, error) {
	if str, ok := value.(string); ok {
		return str, nil
	}

	return "", fnerrors.BadInputError("%s: expected a string", what)
}

func reUnmarshal(value any, target any) error {
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, target)
}

type resourceFieldSyntax struct {
	Resource string `json:"resource"`
	FieldRef string `json:"fieldRef"`
}
