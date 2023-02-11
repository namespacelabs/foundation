// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package args

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing/invariants"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type EnvMap struct {
	Values map[string]ResolvableValue
}

var _ json.Unmarshaler = &EnvMap{}
var _ json.Unmarshaler = &ResolvableValue{}

func (cem *EnvMap) UnmarshalJSON(data []byte) error {
	var values map[string]ResolvableValue
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	cem.Values = values
	return nil
}

func (cem *EnvMap) Parsed(ctx context.Context, pl pkggraph.PackageLoader, owner schema.PackageName) ([]*schema.BinaryConfig_EnvEntry, error) {
	if cem == nil {
		return nil, nil
	}

	var env []*schema.BinaryConfig_EnvEntry
	for key, value := range cem.Values {
		out, err := value.ToProto(ctx, pl, owner)
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

var validKeys = []string{
	"fromSecret",
	"fromField",
	"fromServiceEndpoint",
	"fromServiceIngress",
	"fromResourceField",
	"experimentalFromDownwardsFieldPath",
}

func (ev *ResolvableValue) UnmarshalJSON(data []byte) error {
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

		case "fromServiceIngress":
			ev.fromServiceIngress, err = mustString("fromServiceIngress", m[keys[0]])

		case "fromField":
			var ref fromFieldSyntax
			if err := reUnmarshal(m[keys[0]], &ref); err != nil {
				return fnerrors.BadInputError("failed to parse fromField: %w", err)
			}
			ev.fromField = &ref
			return nil

		case "fromResourceField":
			var ref resourceFieldSyntax
			if err := reUnmarshal(m[keys[0]], &ref); err != nil {
				return fnerrors.BadInputError("failed to parse fromResourceField: %w", err)
			}

			ev.fromResourceField = &ref
			return nil

		case "experimentalFromDownwardsFieldPath":
			ev.experimentalFromDownwardsFieldPath, err = mustString("experimentalFromDownwardsFieldPath", m[keys[0]])
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

type fromFieldSyntax struct {
	Instance *instanceRefSyntax `json:"instance,omitempty"`
	FieldRef string             `json:"fieldRef"`
}

func (ff *fromFieldSyntax) ToProto(ctx context.Context, pl pkggraph.PackageLoader, owner schema.PackageName) (*schema.FieldSelector, error) {
	if ff.Instance == nil {
		return nil, fnerrors.New("instance is required")
	}

	instance, err := toInstanceProto(ctx, pl, owner, ff.Instance)
	if err != nil {
		return nil, err
	}

	return &schema.FieldSelector{Instance: instance, FieldSelector: ff.FieldRef}, nil
}

func toInstanceProto(ctx context.Context, pl pkggraph.PackageLoader, owner schema.PackageName, instance *instanceRefSyntax) (*schema.FieldSelector_Instance, error) {
	if instance.FromService != "" {
		serviceRef, err := schema.ParsePackageRef(owner, instance.FromService)
		if err != nil {
			return nil, err
		}

		if err := invariants.EnsurePackageLoaded(ctx, pl, owner, serviceRef); err != nil {
			return nil, err
		}

		return &schema.FieldSelector_Instance{
			Service: &schema.ServiceRef{
				ServerRef:   &schema.PackageRef{PackageName: serviceRef.PackageName},
				ServiceName: serviceRef.Name,
			},
		}, nil
	}

	if instance.SelectInternalEndpointByKind != "" {
		return &schema.FieldSelector_Instance{
			SelectInternalEndpointByKind: instance.SelectInternalEndpointByKind,
		}, nil
	}

	return nil, fnerrors.New("unknown selector instance")
}

type instanceRefSyntax struct {
	FromService                  string `json:"fromService"`
	SelectInternalEndpointByKind string `json:"selectInternalEndpointByKind"`
}

type resourceFieldSyntax struct {
	Resource string `json:"resource"`
	FieldRef string `json:"fieldRef"`
}
