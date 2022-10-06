// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"context"
	"io/fs"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func MakeLayer(name string, vfs compute.Computable[fs.FS]) NamedLayer {
	return MakeNamedLayer(name, &makeLayer{vfs: vfs, description: name})
}

type makeLayer struct {
	vfs         compute.Computable[fs.FS]
	description string // Does not affect output.

	compute.LocalScoped[Layer]
}

func (m *makeLayer) Inputs() *compute.In {
	return compute.Inputs().Computable("vfs", m.vfs)
}

func (m *makeLayer) Action() *tasks.ActionEvent {
	return tasks.Action("oci.make-layer").Arg("name", m.description)
}

func (m *makeLayer) Compute(ctx context.Context, deps compute.Resolved) (Layer, error) {
	return LayerFromFS(ctx, compute.MustGetDepValue(deps, m.vfs, "vfs"))
}
