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
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type ResolvableValue struct {
	value                              string
	fromSecret                         string
	fromServiceEndpoint                string
	fromServiceIngress                 string
	experimentalFromSecret             string
	experimentalFromDownwardsFieldPath string
	fromField                          *fromFieldSyntax
	fromResourceField                  *resourceFieldSyntax
}

var _ json.Unmarshaler = &ResolvableValue{}

var validKeys = []string{
	"fromSecret",
	"fromField",
	"fromServiceEndpoint",
	"fromServiceIngress",
	"fromResourceField",
	"experimentalFromSecret",
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

		case "experimentalFromSecret":
			ev.experimentalFromSecret, err = mustString("experimentalFromSecret", m[keys[0]])

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

func (value *ResolvableValue) ToProto(ctx context.Context, pl pkggraph.PackageLoader, loc pkggraph.Location) (*schema.Resolvable, error) {
	out := &schema.Resolvable{}

	switch {
	case value.value != "":
		out.Value = value.value

	case value.fromSecret != "":
		secretRef, err := schema.ParsePackageRef(loc, value.fromSecret)
		if err != nil {
			return nil, err
		}
		if err := invariants.EnsurePackageLoaded(ctx, pl, loc, secretRef); err != nil {
			return nil, err
		}

		out.FromSecretRef = secretRef

	case value.fromServiceEndpoint != "":
		serviceRef, err := schema.ParsePackageRef(loc, value.fromServiceEndpoint)
		if err != nil {
			return nil, err
		}
		if err := invariants.EnsurePackageLoaded(ctx, pl, loc, serviceRef); err != nil {
			return nil, err
		}

		out.FromServiceEndpoint = &schema.ServiceRef{
			ServerRef:   &schema.PackageRef{PackageName: serviceRef.PackageName},
			ServiceName: serviceRef.Name,
		}

	case value.fromServiceIngress != "":
		serviceRef, err := schema.ParsePackageRef(loc, value.fromServiceIngress)
		if err != nil {
			return nil, err
		}
		if err := invariants.EnsurePackageLoaded(ctx, pl, loc, serviceRef); err != nil {
			return nil, err
		}
		out.FromServiceIngress = &schema.ServiceRef{
			ServerRef:   &schema.PackageRef{PackageName: serviceRef.PackageName},
			ServiceName: serviceRef.Name,
		}

	case value.fromResourceField != nil:
		resourceRef, err := schema.ParsePackageRef(loc, value.fromResourceField.Resource)
		if err != nil {
			return nil, err
		}
		if err := invariants.EnsurePackageLoaded(ctx, pl, loc, resourceRef); err != nil {
			return nil, err
		}

		out.FromResourceField = &schema.ResourceConfigFieldSelector{
			Resource:      resourceRef,
			FieldSelector: value.fromResourceField.FieldRef,
		}

	case value.fromField != nil:
		x, err := value.fromField.ToProto(ctx, pl, loc)
		if err != nil {
			return nil, err
		}
		out.FromFieldSelector = x

	case value.experimentalFromDownwardsFieldPath != "":
		out.ExperimentalFromDownwardsFieldPath = value.experimentalFromDownwardsFieldPath

	case value.experimentalFromSecret != "":
		out.ExperimentalFromSecret = value.experimentalFromSecret
	}

	return out, nil
}

type fromFieldSyntax struct {
	Instance *instanceRefSyntax `json:"instance,omitempty"`
	FieldRef string             `json:"fieldRef"`
}

func (ff *fromFieldSyntax) ToProto(ctx context.Context, pl pkggraph.PackageLoader, loc pkggraph.Location) (*schema.FieldSelector, error) {
	if ff.Instance == nil {
		return nil, fnerrors.New("instance is required")
	}

	instance, err := toInstanceProto(ctx, pl, loc, ff.Instance)
	if err != nil {
		return nil, err
	}

	return &schema.FieldSelector{Instance: instance, FieldSelector: ff.FieldRef}, nil
}

func toInstanceProto(ctx context.Context, pl pkggraph.PackageLoader, loc pkggraph.Location, instance *instanceRefSyntax) (*schema.ResolvableSource, error) {
	if instance.FromServer != "" {
		ref, err := schema.ParsePackageRef(loc, instance.FromService)
		if err != nil {
			return nil, err
		}

		if err := invariants.EnsurePackageLoaded(ctx, pl, loc, ref); err != nil {
			return nil, err
		}

		return &schema.ResolvableSource{
			Server: ref,
		}, nil
	}

	if instance.FromService != "" {
		serviceRef, err := schema.ParsePackageRef(loc, instance.FromService)
		if err != nil {
			return nil, err
		}

		if err := invariants.EnsurePackageLoaded(ctx, pl, loc, serviceRef); err != nil {
			return nil, err
		}

		return &schema.ResolvableSource{
			Service: &schema.ServiceRef{
				ServerRef:   &schema.PackageRef{PackageName: serviceRef.PackageName},
				ServiceName: serviceRef.Name,
			},
		}, nil
	}

	if instance.SelectInternalEndpointByKind != "" {
		return &schema.ResolvableSource{
			SelectInternalEndpointByKind: instance.SelectInternalEndpointByKind,
		}, nil
	}

	if instance.FromJsonFile != "" {
		fc, err := protos.AllocateFileContents(ctx, protos.ParseContext{
			FS:          loc.Module.ReadOnlyFS(loc.Rel()),
			PackageName: loc.PackageName,
		}, instance.FromJsonFile)
		if err != nil {
			return nil, err
		}

		return &schema.ResolvableSource{
			UntypedJson: fc,
		}, nil
	}

	return nil, fnerrors.New("unknown selector instance")
}

type instanceRefSyntax struct {
	FromServer                   string
	FromService                  string
	SelectInternalEndpointByKind string
	FromJsonFile                 string
}

func (irs *instanceRefSyntax) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	tok, err := d.Token()
	if err != nil {
		return err
	}

	var validKeys = []string{"fromServer", "fromService", "selectInternalEndpointByKind", "fromJsonFile"}

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
		case "fromServer":
			irs.FromServer, err = mustString("fromServer", m[keys[0]])

		case "fromService":
			irs.FromService, err = mustString("fromService", m[keys[0]])

		case "selectInternalEndpointByKind":
			irs.SelectInternalEndpointByKind, err = mustString("SelectInternalEndpointByKind", m[keys[0]])

		case "fromJsonFile":
			irs.FromJsonFile, err = mustString("fromJsonFile", m[keys[0]])
		}

		return err
	}

	return fnerrors.BadInputError("failed to parse value, unexpected token %v", tok)
}

type resourceFieldSyntax struct {
	Resource string `json:"resource"`
	FieldRef string `json:"fieldRef"`
}
