// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"context"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
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
