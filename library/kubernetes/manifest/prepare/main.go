// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"bytes"
	"context"

	"namespacelabs.dev/foundation/framework/kubernetes/kubeparser"
	"namespacelabs.dev/foundation/framework/provisioning"
	"namespacelabs.dev/foundation/library/kubernetes/manifest"
	"namespacelabs.dev/foundation/schema"
)

func main() {
	h := provisioning.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleApply(func(ctx context.Context, req provisioning.StackRequest, out *provisioning.ApplyOutput) error {
		intent := &manifest.AppliedManifestIntent{}
		if err := req.UnpackInput(intent); err != nil {
			return err
		}

		instance := &manifest.AppliedManifestInstance{}
		for _, src := range intent.Sources {
			p := &manifest.AppliedManifestInstance_ParsedFile{}

			applies, err := kubeparser.MultipleFromReader(req.PackageOwner(), bytes.NewReader(src.Contents), false)
			if err != nil {
				return err
			}

			for _, apply := range applies {
				apply.Creator = schema.MakePackageSingleRef(schema.MakePackageName(req.PackageOwner()))

				out.Invocations = append(out.Invocations, apply)
				p.Manifest = append(p.Manifest, &manifest.AppliedManifestInstance_ParsedManifest{
					ApiVersion: apply.Parsed.APIVersion,
					Kind:       apply.Parsed.Kind,
					Namespace:  apply.Parsed.Namespace,
					Name:       apply.Parsed.Name,
				})
			}

			instance.File = append(instance.File, p)
		}

		out.OutputResourceInstance = instance
		return nil
	})
	provisioning.Handle(h)
}
