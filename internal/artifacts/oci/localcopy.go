// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"context"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/std/tasks"
)

func LocalCopy(input NamedImage) NamedImage {
	return MakeNamedImage(input.Description(), &localCopy{desc: input.Description(), source: input.Image()})
}

type localCopy struct {
	desc   string
	source compute.Computable[Image]

	compute.LocalScoped[Image]
}

var _ compute.Computable[Image] = &localCopy{}

func (lc *localCopy) Action() *tasks.ActionEvent {
	return tasks.Action("oci.local-copy").Arg("ref", lc.desc)
}

func (lc *localCopy) Inputs() *compute.In {
	return compute.Inputs().Computable("source", lc.source)
}

func (lc *localCopy) Compute(ctx context.Context, deps compute.Resolved) (Image, error) {
	source := compute.MustGetDepValue(deps, lc.source, "source")
	return EnsureLocal(ctx, source)
}
