// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package helm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	orchpb "namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/go-ids"
)

type parsedHelmInstall struct {
	ID          string
	Namespace   string
	ReleaseName string
	Chart       *chart.Chart
	Values      map[string]any
}

func Register() {
	execution.RegisterVFuncs(execution.VFuncs[*OpHelmInstall, *parsedHelmInstall]{
		EmitStart: func(_ context.Context, _ *schema.SerializedInvocation, apply *parsedHelmInstall, ch chan *orchestration.Event) {
			ch <- makeEvent(apply, orchestration.Event_NOT_READY, orchestration.Event_PLANNED)
		},
		Parse: func(ctx context.Context, _ *schema.SerializedInvocation, apply *OpHelmInstall) (*parsedHelmInstall, error) {
			if apply.ChartArchiveBlob == nil {
				return nil, fnerrors.BadInputError("chart_archive_blob is missing")
			}

			if apply.ReleaseName == "" {
				return nil, fnerrors.BadInputError("release_name is missing")
			}

			chart, err := loader.LoadArchive(bytes.NewReader(apply.ChartArchiveBlob.Inline))
			if err != nil {
				return nil, err
			}

			var values map[string]any
			if apply.Values != nil {
				if err := json.Unmarshal([]byte(apply.Values.Inline), &values); err != nil {
					return nil, fnerrors.New("failed to unserialize values: %w", err)
				}
			}

			ns := apply.Namespace
			if ns == "" {
				nsc, err := kubedef.InjectedKubeClusterNamespace(ctx)
				if err != nil {
					return nil, err
				}

				ns = nsc.KubeConfig().Namespace
			}

			return &parsedHelmInstall{ID: ids.NewRandomBase32ID(8), Chart: chart, Values: values, Namespace: ns, ReleaseName: apply.ReleaseName}, nil
		},
		HandleWithEvents: func(ctx context.Context, d *schema.SerializedInvocation, apply *parsedHelmInstall, ch chan *orchestration.Event) (*execution.HandleResult, error) {
			ch <- makeEvent(apply, orchestration.Event_NOT_READY, orchestration.Event_WAITING)

			cluster, err := kubedef.InjectedKubeCluster(ctx)
			if err != nil {
				return nil, err
			}

			if _, err := NewInstall(ctx, cluster, apply.ReleaseName, apply.Namespace, apply.Chart, apply.Values); err != nil {
				return nil, err
			}

			ch <- makeEvent(apply, orchestration.Event_READY, orchestration.Event_DONE)

			return nil, nil
		},
	})
}

func makeEvent(apply *parsedHelmInstall, ready orchpb.Event_ReadyTriState, stage orchpb.Event_Stage) *orchpb.Event {
	return &orchpb.Event{
		ResourceId:    apply.ID,
		Category:      "Helm charts",
		Ready:         ready,
		Stage:         stage,
		ResourceLabel: fmt.Sprintf("%s (%s %s)", apply.ReleaseName, apply.Chart.Metadata.Name, apply.Chart.Metadata.Version),
	}
}
