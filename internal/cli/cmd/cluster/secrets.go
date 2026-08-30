// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"strings"

	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/secrets/secretsprovider"
	"github.com/pkg/errors"
	"github.com/tonistiigi/go-csvvalue"
)

// buildSecret mirrors the subset of github.com/docker/buildx/util/buildflags.Secret
// that nsc relies on, so that we don't have to depend on the docker/buildx module.
type buildSecret struct {
	ID       string
	FilePath string
	Env      string
}

// parseSecretSpecs replicates buildflags.ParseSecretSpecs from docker/buildx.
func parseSecretSpecs(sl []string) ([]buildSecret, error) {
	fs := make([]buildSecret, 0, len(sl))
	for _, v := range sl {
		if v == "" {
			continue
		}

		s, err := parseSecret(v)
		if err != nil {
			return nil, err
		}
		fs = append(fs, s)
	}
	return fs, nil
}

func parseSecret(value string) (buildSecret, error) {
	fields, err := csvvalue.Fields(value, nil)
	if err != nil {
		return buildSecret{}, errors.Wrap(err, "failed to parse csv secret")
	}

	var s buildSecret
	var typ string
	for _, field := range fields {
		parts := strings.SplitN(field, "=", 2)
		key := strings.ToLower(parts[0])

		if len(parts) != 2 {
			return buildSecret{}, errors.Errorf("invalid field '%s' must be a key=value pair", field)
		}

		value := parts[1]
		switch key {
		case "type":
			if value != "file" && value != "env" {
				return buildSecret{}, errors.Errorf("unsupported secret type %q", value)
			}
			typ = value
		case "id":
			s.ID = value
		case "source", "src":
			s.FilePath = value
		case "env":
			s.Env = value
		default:
			return buildSecret{}, errors.Errorf("unexpected key '%s' in '%s'", key, field)
		}
	}
	if typ == "env" && s.Env == "" {
		s.Env = s.FilePath
		s.FilePath = ""
	}
	return s, nil
}

// createSecretsProvider replicates build.CreateSecrets from docker/buildx.
func createSecretsProvider(secrets []buildSecret) (session.Attachable, error) {
	fs := make([]secretsprovider.Source, 0, len(secrets))
	for _, secret := range secrets {
		fs = append(fs, secretsprovider.Source{
			ID:       secret.ID,
			FilePath: secret.FilePath,
			Env:      secret.Env,
		})
	}
	store, err := secretsprovider.NewStore(fs)
	if err != nil {
		return nil, err
	}
	return secretsprovider.NewSecretProvider(store), nil
}
