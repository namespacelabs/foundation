// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package runtime

import (
	"encoding/json"
	"fmt"
	"os"

	"namespacelabs.dev/foundation/schema/runtime"
)

type Server = runtime.Server

func LoadRuntimeConfig() (*runtime.RuntimeConfig, error) {
	configBytes, err := os.ReadFile("/namespace/config/runtime.json")
	if err != nil {
		return nil, fmt.Errorf("failed to unwrap runtime configuration: %w", err)
	}

	rt := &runtime.RuntimeConfig{}
	if err := json.Unmarshal(configBytes, rt); err != nil {
		return nil, fmt.Errorf("failed to unmarshal runtime configuration: %w", err)
	}

	return rt, nil
}

func LoadBuildVCS() (*runtime.BuildVCS, error) {
	serializedVCS, err := os.ReadFile("/namespace/config/buildvcs.json")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to load BuildVCS: %w", err)
	}

	vcs := &runtime.BuildVCS{}
	if err := json.Unmarshal(serializedVCS, vcs); err != nil {
		return nil, fmt.Errorf("failed to parse BuildVCS: %w", err)
	}

	return vcs, nil
}

func Endpoint(srv *Server, name string) (string, error) {
	for _, s := range srv.Service {
		if s.Name == name {
			return s.Endpoint, nil
		}
	}

	return "", fmt.Errorf("endpoint %s not found for server %s", name, srv.PackageName)
}

func ServerEndpoint(rtcfg *runtime.RuntimeConfig, pkg, name string) (string, error) {
	for _, e := range rtcfg.StackEntry {
		if e.PackageName == pkg {
			return Endpoint(e, name)
		}
	}

	return "", fmt.Errorf("server %s not found in runtime config stack", pkg)
}
