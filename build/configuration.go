// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package build

import (
	"fmt"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
)

type buildTarget struct {
	target *specs.Platform
	name   compute.Computable[oci.AllocatedName]
}

type buildConfiguration struct {
	*buildTarget
	source    schema.PackageName
	label     string
	workspace Workspace
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

func (c *buildTarget) TargetPlatform() *specs.Platform                    { return c.target }
func (c *buildTarget) PublishName() compute.Computable[oci.AllocatedName] { return c.name }
func (d *buildConfiguration) SourcePackage() schema.PackageName           { return d.source }
func (d *buildConfiguration) SourceLabel() string                         { return d.label }
func (d *buildConfiguration) Workspace() Workspace                        { return d.workspace }
