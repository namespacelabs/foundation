// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Forked from github.com/docker/buildx/util/dockerutil/context.go and
// store/storeutil/storeutil.go (v0.32.1).

package buildxstore

import (
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/context/docker"
	"github.com/pkg/errors"
)

// GetStore returns the current builder instance store.
func GetStore(dockerCli command.Cli) (*Txn, func(), error) {
	s, err := New(NewConfig(dockerCli))
	if err != nil {
		return nil, nil, err
	}
	return s.Txn()
}

// getDockerEndpoint returns docker endpoint metadata for the given context.
func getDockerEndpoint(dockerCli command.Cli, name string) (*docker.EndpointMeta, error) {
	list, err := dockerCli.ContextStore().List()
	if err != nil {
		return nil, err
	}
	for _, l := range list {
		if l.Name == name {
			epm, err := docker.EndpointFromContext(l)
			if err != nil {
				return nil, err
			}
			return &epm, nil
		}
	}
	return nil, nil
}

// GetCurrentEndpoint returns the current default endpoint value.
func GetCurrentEndpoint(dockerCli command.Cli) (string, error) {
	name := dockerCli.CurrentContext()
	if name != "default" {
		return name, nil
	}
	dem, err := getDockerEndpoint(dockerCli, name)
	if err != nil {
		return "", errors.Errorf("docker endpoint for %q not found", name)
	} else if dem != nil {
		return dem.Host, nil
	}
	return "", nil
}
