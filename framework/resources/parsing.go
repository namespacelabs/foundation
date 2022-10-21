// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package resources

import (
	"encoding/json"
	"fmt"
	"os"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/library/runtime"
)

type Parser struct {
	data []byte
}

func NewParser(data []byte) *Parser {
	return &Parser{data: data}
}

func (p *Parser) Decode(resource string, out any) error {
	resources := make(map[string]any)
	if err := json.Unmarshal(p.data, &resources); err != nil {
		return err
	}

	val, ok := resources[resource]
	if !ok {
		return fnerrors.InternalError("no resource config found for resource %q", resource)
	}

	// TODO use json decoder to avoid this marshal
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, out)
}

func (p *Parser) ReadSecret(resource string) (string, error) {
	secret := &runtime.SecretInstance{}
	if err := p.Decode(resource, &secret); err != nil {
		return "", err
	}

	if secret.Path == "" {
		return "", fmt.Errorf("secret %s is missing a path to read from", resource)
	}

	data, err := os.ReadFile(secret.Path)
	if err != nil {
		return "", fmt.Errorf("failed to read secret %s from path %s: %w", resource, secret.Path, err)
	}

	return string(data), nil
}
