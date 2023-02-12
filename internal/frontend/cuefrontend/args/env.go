// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package args

import (
	"context"
	"encoding/json"
	"strings"

	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type EnvMap struct {
	Values map[string]ResolvableValue
}

var _ json.Unmarshaler = &EnvMap{}

func (cem *EnvMap) UnmarshalJSON(data []byte) error {
	var values map[string]ResolvableValue
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	cem.Values = values
	return nil
}

func (cem *EnvMap) Parsed(ctx context.Context, pl pkggraph.PackageLoader, loc pkggraph.Location) ([]*schema.BinaryConfig_EnvEntry, error) {
	if cem == nil {
		return nil, nil
	}

	var env []*schema.BinaryConfig_EnvEntry
	for key, value := range cem.Values {
		out, err := value.ToProto(ctx, pl, loc)
		if err != nil {
			return nil, err
		}

		env = append(env, &schema.BinaryConfig_EnvEntry{Name: key, Value: out})
	}

	slices.SortFunc(env, func(a, b *schema.BinaryConfig_EnvEntry) bool {
		return strings.Compare(a.Name, b.Name) < 0
	})

	return env, nil
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
