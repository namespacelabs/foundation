// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"fmt"
	"strings"

	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/secrets/secretsprovider"
	csvvalue "github.com/tonistiigi/go-csvvalue"
)

// buildSecret describes a secret exposed to a build, parsed from a `--secret`
// CLI spec (e.g. "id=mysecret,src=/path" or "id=mysecret,env=VAR"). This mirrors
// the subset of github.com/docker/buildx/util/buildflags that we rely on.
type buildSecret struct {
	ID       string
	FilePath string
	Env      string
}

func parseSecretSpecs(specs []string) ([]buildSecret, error) {
	out := make([]buildSecret, 0, len(specs))
	for _, v := range specs {
		if v == "" {
			continue
		}

		s, err := parseSecretSpec(v)
		if err != nil {
			return nil, err
		}

		out = append(out, s)
	}

	return out, nil
}

func parseSecretSpec(value string) (buildSecret, error) {
	fields, err := csvvalue.Fields(value, nil)
	if err != nil {
		return buildSecret{}, fmt.Errorf("failed to parse csv secret: %w", err)
	}

	var s buildSecret
	var typ string
	for _, field := range fields {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 {
			return buildSecret{}, fmt.Errorf("invalid field '%s' must be a key=value pair", field)
		}

		key := strings.ToLower(parts[0])
		val := parts[1]
		switch key {
		case "type":
			if val != "file" && val != "env" {
				return buildSecret{}, fmt.Errorf("unsupported secret type %q", val)
			}
			typ = val
		case "id":
			s.ID = val
		case "source", "src":
			s.FilePath = val
		case "env":
			s.Env = val
		default:
			return buildSecret{}, fmt.Errorf("unexpected key '%s' in '%s'", key, field)
		}
	}

	if typ == "env" && s.Env == "" {
		s.Env = s.FilePath
		s.FilePath = ""
	}

	return s, nil
}

// createSecretsProvider builds a buildkit session attachable from the parsed
// secrets. This replaces github.com/docker/buildx/build.CreateSecrets.
func createSecretsProvider(secrets []buildSecret) (session.Attachable, error) {
	sources := make([]secretsprovider.Source, 0, len(secrets))
	for _, s := range secrets {
		sources = append(sources, secretsprovider.Source{
			ID:       s.ID,
			FilePath: s.FilePath,
			Env:      s.Env,
		})
	}

	store, err := secretsprovider.NewStore(sources)
	if err != nil {
		return nil, err
	}

	return secretsprovider.NewSecretProvider(store), nil
}
