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
	name   compute.Computable[oci.AllocatedName]
}

type buildConfiguration struct {
	*buildTarget
	source          schema.PackageName
	label           string
	workspace       Workspace
	prefersBuildkit bool
}

func NewBuildTarget(target *specs.Platform) *buildTarget {
	return &buildTarget{target: target}
}

func (c *buildTarget) WithTargetName(name compute.Computable[oci.AllocatedName]) *buildTarget {
	c.name = name
	return c
}

func (c *buildTarget) WithSourcePackage(pkg schema.PackageName) *buildConfiguration {
	d := buildConfiguration{buildTarget: c}
	return d.WithSourcePackage(pkg)
}

func (c *buildTarget) WithSourceLabel(format string, args ...any) *buildConfiguration {
	d := buildConfiguration{buildTarget: c}
	return d.WithSourceLabel(format, args...)
}

func (c *buildTarget) WithWorkspace(w Workspace) *buildConfiguration {
	d := buildConfiguration{buildTarget: c}
	return d.WithWorkspace(w)
}

func (d *buildConfiguration) WithPrefersBuildkit(prefers bool) *buildConfiguration {
	d.prefersBuildkit = prefers
	return d
}

func (d *buildConfiguration) WithWorkspace(w Workspace) *buildConfiguration {
	d.workspace = w
	return d
}

func (d *buildConfiguration) WithSourcePackage(pkg schema.PackageName) *buildConfiguration {
	d.source = pkg
	return d
}

func (d *buildConfiguration) WithSourceLabel(format string, args ...any) *buildConfiguration {
	d.label = fmt.Sprintf(format, args...)
	return d
}

func CopyConfiguration(b Configuration) *buildConfiguration {
	t := NewBuildTarget(b.TargetPlatform())
	if x := b.PublishName(); x != nil {
		t = t.WithTargetName(x)
	}

	return t.WithSourcePackage(b.SourcePackage()).WithSourceLabel(b.SourceLabel()).WithPrefersBuildkit(b.PrefersBuildkit()).WithWorkspace(b.Workspace())
}

func (c *buildTarget) TargetPlatform() *specs.Platform                    { return c.target }
func (c *buildTarget) PublishName() compute.Computable[oci.AllocatedName] { return c.name }
func (d *buildConfiguration) SourcePackage() schema.PackageName           { return d.source }
func (d *buildConfiguration) SourceLabel() string                         { return d.label }
func (d *buildConfiguration) PrefersBuildkit() bool                       { return d.prefersBuildkit }
func (d *buildConfiguration) Workspace() Workspace                        { return d.workspace }
