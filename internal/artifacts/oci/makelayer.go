// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"context"
	"io/fs"

	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type MakeLayerOpts struct {
	Label string
}

func MakeLayer(name string, vfs compute.Computable[fs.FS]) compute.Computable[Layer] {
	return &makeLayer{vfs: vfs, opts: MakeLayerOpts{Label: name}}
}

func MakeLayerWithOpts(vfs compute.Computable[fs.FS], opts MakeLayerOpts) compute.Computable[Layer] {
	return &makeLayer{vfs: vfs, opts: opts}
}

type makeLayer struct {
	vfs  compute.Computable[fs.FS]
	opts MakeLayerOpts

	compute.LocalScoped[Layer]
}

func (m *makeLayer) Inputs() *compute.In {
	return compute.Inputs().Computable("vfs", m.vfs)
}

func (m *makeLayer) Action() *tasks.ActionEvent {
	return tasks.Action("oci.make-layer").Arg("label", m.opts.Label)
}

func (m *makeLayer) Compute(ctx context.Context, deps compute.Resolved) (Layer, error) {
	return LayerFromFS(ctx, compute.GetDepValue(deps, m.vfs, "vfs"))
}