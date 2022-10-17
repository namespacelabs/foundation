// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

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
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
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

			// XXX use watch.
			return client.PollImmediateWithContext(ctx, 500*time.Millisecond, 5*time.Minute, func(c context.Context) (bool, error) {
				opts := metav1.GetOptions{}

				req := cli.Get().Resource(w.Resource.Resource).Namespace(w.Namespace).Name(w.Name).Body(&opts)

				var res unstructured.Unstructured
				if err := req.Do(ctx).Into(&res); err != nil {
					if errors.IsNotFound(err) {
						return false, nil
					} else {
						return false, err
					}
				}

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
		})
}
