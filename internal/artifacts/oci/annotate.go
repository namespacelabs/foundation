// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"context"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/std/tasks"
)

func AnnotateImage(src NamedImage, anns map[string]string) compute.Computable[Image] {
	return &annotateImage{src: src, anns: anns}
}

type annotateImage struct {
	src  NamedImage
	anns map[string]string
	compute.LocalScoped[Image]
}

func (al *annotateImage) Action() *tasks.ActionEvent {
	return tasks.Action("oci.annotate-image").Arg("src", al.src.Description())
}

func (al *annotateImage) Inputs() *compute.In {
	return compute.Inputs().Computable("src", al.src.Image()).StrMap("anns", al.anns)
}

func (al *annotateImage) Compute(ctx context.Context, deps compute.Resolved) (Image, error) {
	image, _ := compute.GetDep(deps, al.src.Image(), "src")
	return mutate.Annotations(image.Value, al.anns).(v1.Image), nil
}
