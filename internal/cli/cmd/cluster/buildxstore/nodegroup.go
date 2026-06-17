// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Forked from github.com/docker/buildx/store/nodegroup.go (v0.32.1). The unused
// Leave/Copy helpers were dropped. nsc never passes a buildkitd config file to
// Update, so that branch is rejected here instead of pulling in the upstream
// confutil.LoadConfigFiles dependency.

package buildxstore

import (
	"fmt"
	"time"

	"github.com/containerd/platforms"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type NodeGroup struct {
	Name    string
	Driver  string
	Nodes   []Node
	Dynamic bool

	// skip the following fields from being saved in the store
	DockerContext bool      `json:"-"`
	LastActivity  time.Time `json:"-"`
}

type Node struct {
	Name           string
	Endpoint       string
	Platforms      []ocispecs.Platform
	DriverOpts     map[string]string
	BuildkitdFlags []string `json:"Flags"` // keep the field name for backward compatibility

	Files map[string][]byte
}

func (ng *NodeGroup) Update(name, endpoint string, platforms []string, endpointsSet bool, actionAppend bool, buildkitdFlags []string, buildkitdConfigFile string, do map[string]string) error {
	if ng.Dynamic {
		return errors.New("dynamic node group does not support Update")
	}
	if buildkitdConfigFile != "" {
		return errors.New("buildkitd config files are not supported by this buildx store fork")
	}
	i := ng.findNode(name)
	if i == -1 && !actionAppend {
		if len(ng.Nodes) > 0 {
			return errors.Errorf("node %s not found, did you mean to append?", name)
		}
		ng.Nodes = nil
	}

	pp, err := parsePlatforms(platforms)
	if err != nil {
		return err
	}

	if i != -1 {
		n := ng.Nodes[i]
		needsRestart := false
		if endpointsSet {
			n.Endpoint = endpoint
			needsRestart = true
		}
		if len(platforms) > 0 {
			n.Platforms = pp
		}
		if buildkitdFlags != nil {
			n.BuildkitdFlags = buildkitdFlags
			needsRestart = true
		}
		if do != nil {
			n.DriverOpts = do
			needsRestart = true
		}
		if needsRestart {
			logrus.Warn("new settings may not be used until builder is restarted")
		}

		ng.Nodes[i] = n
		return ng.validateDuplicates(endpoint, i)
	}

	if name == "" {
		name = ng.nextNodeName()
	}

	name, err = ValidateName(name)
	if err != nil {
		return err
	}

	n := Node{
		Name:           name,
		Endpoint:       endpoint,
		Platforms:      pp,
		DriverOpts:     do,
		BuildkitdFlags: buildkitdFlags,
	}

	ng.Nodes = append(ng.Nodes, n)
	return ng.validateDuplicates(endpoint, len(ng.Nodes)-1)
}

func (ng *NodeGroup) validateDuplicates(ep string, idx int) error {
	i := 0
	for _, n := range ng.Nodes {
		if n.Endpoint == ep {
			i++
		}
	}
	if i > 1 {
		return errors.Errorf("invalid duplicate endpoint %s", ep)
	}

	m := map[string]struct{}{}
	for _, p := range ng.Nodes[idx].Platforms {
		m[platforms.Format(p)] = struct{}{}
	}

	for i := range ng.Nodes {
		if i == idx {
			continue
		}
		ng.Nodes[i].Platforms = filterPlatforms(ng.Nodes[i].Platforms, m)
	}

	return nil
}

func (ng *NodeGroup) findNode(name string) int {
	for i, n := range ng.Nodes {
		if n.Name == name {
			return i
		}
	}
	return -1
}

func (ng *NodeGroup) nextNodeName() string {
	i := 0
	for {
		name := fmt.Sprintf("%s%d", ng.Name, i)
		if ii := ng.findNode(name); ii != -1 {
			i++
			continue
		}
		return name
	}
}

func filterPlatforms(in []ocispecs.Platform, m map[string]struct{}) []ocispecs.Platform {
	out := make([]ocispecs.Platform, 0, len(in))
	for _, p := range in {
		if _, ok := m[platforms.Format(p)]; !ok {
			out = append(out, p)
		}
	}
	return out
}
