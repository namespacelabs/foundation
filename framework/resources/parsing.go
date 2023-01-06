// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package resources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	"google.golang.org/grpc/codes"
	"namespacelabs.dev/foundation/framework/rpcerrors"
)

type Parsed struct {
	resources map[string]any
}

func LoadResources() (*Parsed, error) {
	configBytes, err := os.ReadFile("/namespace/config/resources.json")
	if err != nil {
		return nil, fmt.Errorf("failed to unwrap resource configuration: %w", err)
	}

	return ParseResourceData(configBytes)
}

func ParseResourceData(data []byte) (*Parsed, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	tok, err := dec.Token()
	if err == nil && tok != json.Delim('{') {
		err = fmt.Errorf("expected an object, got %v", tok)
	}

	if err != nil {
		return nil, fmt.Errorf("bad resource data: %w", err)
	}

	resources := map[string]any{}
	for dec.More() {
		label, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("bad resource data: %w", err)
		}

		strLabel, ok := label.(string)
		if !ok {
			return nil, fmt.Errorf("bad resource data, expected label: %w", err)
		}

		var raw any
		if err := dec.Decode(&raw); err != nil {
			return nil, fmt.Errorf("bad resource data: failed to decode: %w", err)
		}

		resources[strLabel] = raw
	}

	finalTok, err := dec.Token()
	if err == nil && finalTok != json.Delim('}') {
		err = fmt.Errorf("expected object closure, got %v", finalTok)
	}

	if err != nil {
		return nil, fmt.Errorf("bad resource data: %w", err)
	}

	return &Parsed{resources: resources}, nil
}

// SelectField traverses the specified resource, if one exists, and obtains the
// value specified by the field selector.
func (p *Parsed) SelectField(resource, field string) (any, error) {
	raw, ok := p.resources[resource]
	if !ok {
		return nil, rpcerrors.Errorf(codes.NotFound, "no instance found for resource %q", resource)
	}

	return selectField(resource, field, raw, field)
}

func selectField(resource, originalSel string, value any, field string) (any, error) {
	if field == "" {
		switch x := value.(type) {
		// Hack! Guess the primitive number type.
		case json.Number:
			if n, err := x.Int64(); err == nil {
				return n, nil
			}

			return x.Float64()

		default:
			return value, nil
		}
	}

	p := strings.SplitN(field, ".", 2)

	fieldName := p[0]
	left := ""
	if len(p) > 1 {
		left = p[1]
	}

	if field == "" {
		return nil, rpcerrors.Errorf(codes.InvalidArgument, "%s: invalid field selector", resource)
	}

	// XXX this field parsing is fairly simple, it only traverses maps. We should also:
	//  - support arbitrary keys (e.g. with dots)
	//  - support indexing.

	switch x := value.(type) {
	case map[string]interface{}:
		if child, ok := x[fieldName]; ok {
			if child == nil {
				return nil, rpcerrors.Errorf(codes.NotFound, "%s: %s: no value set", resource, originalSel)
			}

			return selectField(resource, originalSel, child, left)
		}

		return nil, rpcerrors.Errorf(codes.NotFound, "%s: %s: selector doesn't match a value", resource, originalSel)
	}

	return nil, rpcerrors.Errorf(codes.InvalidArgument, "%s: resource is of type %q, not supported", resource, reflect.TypeOf(value).String())
}

func (p *Parsed) Unmarshal(resource string, out any) error {
	raw, ok := p.resources[resource]
	if !ok {
		return rpcerrors.Errorf(codes.NotFound, "no resource config found for resource %q", resource)
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return rpcerrors.Errorf(codes.Internal, "%s: failed to re-marshal value: %w", resource, err)
	}

	if err := json.Unmarshal(data, out); err != nil {
		return rpcerrors.Errorf(codes.Internal, "%s: failed to unmarshal resource: %w", resource, err)
	}

	return nil
}
