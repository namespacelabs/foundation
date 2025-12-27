// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubeobserver

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/tasks"
)

type WaitOnGenerationCondition struct {
	RestConfig         *rest.Config
	Name, Namespace    string
	ExpectedGeneration int64
	ConditionType      string
	Resource           schema.GroupVersionResource
}

func (w WaitOnGenerationCondition) WaitUntilReady(ctx context.Context, ch chan *orchestration.Event) error {
	if ch != nil {
		defer close(ch)
	}

	return tasks.Action("kubernetes.wait-on-condition").
		Arg("group_version", w.Resource.GroupVersion()).
		Arg("resource", w.Resource.Resource).
		Arg("name", w.Name).
		Arg("namespace", w.Namespace).
		Run(ctx, func(ctx context.Context) error {
			cli, err := client.MakeGroupVersionBasedClient(ctx, w.RestConfig, w.Resource.GroupVersion())
			if err != nil {
				return err
			}

			var lastObservedGen int64
			var lastConditionType string
			var resourceFound bool

			pollErr := client.PollImmediateWithContext(ctx, 500*time.Millisecond, 5*time.Minute, func(c context.Context) (bool, error) {
				opts := metav1.GetOptions{}

				req := cli.Get().Resource(w.Resource.Resource).Namespace(w.Namespace).Name(w.Name).Body(&opts)

				var res unstructured.Unstructured
				if err := req.Do(ctx).Into(&res); err != nil {
					if errors.IsNotFound(err) {
						resourceFound = false
						return false, nil
					} else {
						return false, err
					}
				}

				resourceFound = true

				conditions, has, err := unstructured.NestedSlice(res.Object, "status", "conditions")
				if err != nil {
					return false, err
				}

				if !has {
					return false, err
				}

				for _, cond := range conditions {
					if m, ok := cond.(map[string]interface{}); ok {
						gen, has1, err1 := unstructured.NestedInt64(m, "observedGeneration")
						typ, has2, err2 := unstructured.NestedString(m, "type")

						if err1 == nil && err2 == nil && has1 && has2 {
							lastObservedGen = gen
							lastConditionType = typ
							return w.ExpectedGeneration == gen && w.ConditionType == typ, nil
						}

						errs := []error{err1, err2}
						if !has1 {
							errs = append(errs, fnerrors.BadInputError("expected observedGeneration"))
						}
						if !has2 {
							errs = append(errs, fnerrors.BadInputError("expected type"))
						}

						fmt.Fprintf(console.Debug(ctx), "WaitOnGenerationCondition cycle failed: %v\n", multierr.New(errs...))
					}
				}

				return false, nil
			})

			if pollErr != nil {
				return formatConditionWaitError(pollErr, w.Resource, w.Namespace, w.Name, w.ConditionType, w.ExpectedGeneration,
					lastObservedGen, lastConditionType, resourceFound)
			}
			return nil
		})
}

func formatConditionWaitError(err error, resource schema.GroupVersionResource, namespace, name, expectedCondition string,
	expectedGen, observedGen int64, lastConditionType string, resourceFound bool) error {

	resourceID := fmt.Sprintf("%s/%s", namespace, name)

	var statusDetails string
	if !resourceFound {
		statusDetails = "resource not found"
	} else if lastConditionType != "" {
		statusDetails = fmt.Sprintf("condition=%q, generation=%d", lastConditionType, observedGen)
	} else {
		statusDetails = fmt.Sprintf("generation=%d, no matching condition found", observedGen)
	}

	msg := fmt.Sprintf("%s %q: timed out waiting for condition %q\n  Status: %s", resource.Resource, resourceID, expectedCondition, statusDetails)
	msg += fmt.Sprintf("\n  Expected: condition=%q at generation=%d", expectedCondition, expectedGen)
	msg += fmt.Sprintf("\n  Help: kubectl -n %s describe %s %s", namespace, resource.Resource, name)

	return fnerrors.New(msg)
}
