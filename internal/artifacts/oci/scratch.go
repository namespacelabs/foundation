// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"context"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func Scratch() NamedImage {
	return MakeNamedImage("scratch", scratch{})
}

type scratch struct {
	compute.PrecomputeScoped[Image]
}

var _ compute.Digestible = scratch{}

func (scratch) Action() *tasks.ActionEvent { return tasks.Action("oci.make-scratch").LogLevel(2) }
func (scratch) Inputs() *compute.In        { return compute.Inputs().Indigestible("scratch", "scratch") }
func (scratch) Compute(_ context.Context, _ compute.Resolved) (Image, error) {
	return empty.Image, nil
}

func (scratch) ComputeDigest(context.Context) (schema.Digest, error) {
	h, err := empty.Image.Digest()
	return schema.Digest(h), err
}
