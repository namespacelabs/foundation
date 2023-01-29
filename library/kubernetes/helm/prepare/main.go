// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/framework/provisioning"
	"namespacelabs.dev/foundation/internal/fnerrors"
	helmc "namespacelabs.dev/foundation/internal/runtime/kubernetes/helm"
	"namespacelabs.dev/foundation/library/kubernetes/helm"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution/defs"
)

func main() {
	h := provisioning.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleApply(func(ctx context.Context, req provisioning.StackRequest, out *provisioning.ApplyOutput) error {
		intent := &helm.HelmReleaseIntent{}
		if err := req.UnpackInput(intent); err != nil {
			return err
		}

		if intent.Chart == nil {
			return fnerrors.New("chart is required")
		}

		install := &helmc.OpHelmInstall{
			ChartArchiveBlob: &helmc.Blob{
				Inline: intent.Chart.Contents,
			},
			Namespace:   intent.Namespace,
			ReleaseName: intent.ReleaseName,
		}

		if intent.Values != nil {
			install.Values = &helmc.JsonBlob{
				Inline: intent.Values.InlineJson,
			}
		}

		out.Invocations = append(out.Invocations, defs.Static(fmt.Sprintf("Helm Release %q", intent.ReleaseName), install))

		out.OutputResourceInstance = &helm.HelmReleaseInstance{}
		return nil
	})
	provisioning.Handle(h)
}
