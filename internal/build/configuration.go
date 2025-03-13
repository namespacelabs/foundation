// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package build

import (
	"fmt"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/schema"
)

type buildTarget struct {
	target *specs.Platform
	name   compute.Computable[oci.RepositoryWithParent]
}

type buildConfiguration struct {
	buildTarget *buildTarget
	source      schema.PackageName
	label       string
	workspace   Workspace
}

func NewBuildTarget(target *specs.Platform) *buildTarget {
	return &buildTarget{target: target}
}

func (c *buildTarget) WithTargetName(name compute.Computable[oci.RepositoryWithParent]) *buildTarget {
	c.name = name
	return c
}

func (c *buildTarget) WithSourcePackage(pkg schema.PackageName) *buildConfiguration {
	d := buildConfiguration{buildTarget: c}
	return d.WithSourcePackage(pkg)
}

func (c *buildTarget) WithSourceLabel(label string) *buildConfiguration {
	d := buildConfiguration{buildTarget: c}
	return d.WithSourceLabel(label)
}

func (c *buildTarget) WithSourceLabelf(format string, args ...any) *buildConfiguration {
	d := buildConfiguration{buildTarget: c}
	return d.WithSourceLabel(fmt.Sprintf(format, args...))
}

func (c *buildTarget) WithWorkspace(w Workspace) *buildConfiguration {
	d := buildConfiguration{buildTarget: c}
	return d.WithWorkspace(w)
}

func (d *buildConfiguration) WithWorkspace(w Workspace) *buildConfiguration {
	d.workspace = w
	return d
}

func (d *buildConfiguration) WithSourcePackage(pkg schema.PackageName) *buildConfiguration {
	d.source = pkg
	return d
}

func (d *buildConfiguration) WithSourceLabel(label string) *buildConfiguration {
	d.label = label
	return d
}

func (d *buildConfiguration) WithTarget(target *specs.Platform) *buildConfiguration {
	d.buildTarget.target = target
	return d
}

func CopyConfiguration(b Configuration) *buildConfiguration {
	t := NewBuildTarget(b.TargetPlatform())
	if x := b.PublishName(); x != nil {
		t = t.WithTargetName(x)
	}

	return t.WithSourcePackage(b.SourcePackage()).
		WithSourceLabel(b.SourceLabel()).
		WithWorkspace(b.Workspace())
}

func (c *buildTarget) TargetPlatform() *specs.Platform                           { return c.target }
func (c *buildTarget) PublishName() compute.Computable[oci.RepositoryWithParent] { return c.name }

func (d *buildConfiguration) TargetPlatform() *specs.Platform { return d.buildTarget.target }
func (d *buildConfiguration) PublishName() compute.Computable[oci.RepositoryWithParent] {
	return d.buildTarget.name
}
func (d *buildConfiguration) SourcePackage() schema.PackageName { return d.source }
func (d *buildConfiguration) SourceLabel() string               { return d.label }
func (d *buildConfiguration) Workspace() Workspace              { return d.workspace }
