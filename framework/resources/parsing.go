// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package resources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
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

func (p *Parsed) Unmarshal(resource string, out any) error {
	raw, ok := p.resources[resource]
	if !ok {
		return fmt.Errorf("no resource config found for resource %q", resource)
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("%s: failed to re-marshal value: %w", resource, err)
	}

	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("%s: failed to unmarshal resource: %w", resource, err)
	}

	return nil
}
