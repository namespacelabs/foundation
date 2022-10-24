// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubeobserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeschema "k8s.io/apimachinery/pkg/runtime/schema"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/tasks"
)

type ConditionWaiter[Client any] interface {
	Prepare(context.Context, Client) error
	Poll(context.Context, Client) (bool, error)
}

func WaitForCondition[Client any](ctx context.Context, cli Client, action *tasks.ActionEvent, waiter ConditionWaiter[Client]) error {
	return action.Run(ctx, func(ctx context.Context) error {
		if err := waiter.Prepare(ctx, cli); err != nil {
			return err
		}

		return client.PollImmediateWithContext(ctx, 500*time.Millisecond, 5*time.Minute, func(ctx context.Context) (bool, error) {
			return waiter.Poll(ctx, cli)
		})
	})
}

func PrepareEvent(gvk kubeschema.GroupVersionKind, namespace, name, desc string, deployable runtime.Deployable) *orchestration.Event {
	ev := &orchestration.Event{
		ResourceId:          fmt.Sprintf("%s/%s", namespace, name),
		RuntimeSpecificHelp: fmt.Sprintf("kubectl -n %s describe %s %s", namespace, strings.ToLower(gvk.Kind), name),
		Ready:               orchestration.Event_NOT_READY,
	}

	switch {
	case kubedef.IsGVKDeployment(gvk), kubedef.IsGVKStatefulSet(gvk), kubedef.IsGVKPod(gvk):
		ev.Category = "Servers deployed"
	default:
		ev.Category = desc
	}

	if deployable != nil {
		ev.Scope = deployable.GetPackageName()
	}

	return ev
}

type WaitOnResource struct {
	RestConfig *rest.Config

	Name, Namespace  string
	Description      string
	GroupVersionKind kubeschema.GroupVersionKind
	Scope            schema.PackageName

	PreviousGen, ExpectedGen int64
}

func (w WaitOnResource) WaitUntilReady(ctx context.Context, ch chan *orchestration.Event) error {
	if ch != nil {
		defer close(ch)
	}

	cli, err := k8s.NewForConfig(w.RestConfig)
	if err != nil {
		return err
	}

	ev := tasks.Action(strings.ToLower(w.GroupVersionKind.Kind) + ".wait")
	if w.Scope != "" {
		ev = ev.Scope(w.Scope)
	} else {
		ev = ev.Arg("kind", w.GroupVersionKind.Kind).Arg("name", w.Name).Arg("namespace", w.Namespace)
	}

	return ev.Run(ctx, func(ctx context.Context) error {
		ev := PrepareEvent(w.GroupVersionKind, w.Namespace, w.Name, w.Description, nil)
		ev.Scope = w.Scope.String()
		if w.PreviousGen > 0 && w.PreviousGen == w.ExpectedGen {
			ev.AlreadyExisted = true
		}

		if ch != nil {
			ch <- ev
		}

		return client.PollImmediateWithContext(ctx, 500*time.Millisecond, 5*time.Minute, func(c context.Context) (done bool, err error) {
			var observedGeneration int64
			var readyReplicas, replicas, updatedReplicas int32

			switch {
			case kubedef.IsGVKDeployment(w.GroupVersionKind):
				res, err := cli.AppsV1().Deployments(w.Namespace).Get(c, w.Name, metav1.GetOptions{})
				if err != nil {
					// If the resource is not visible yet, wait anyway, as the
					// only way to get here is by requesting that the resource
					// be created.
					if errors.IsNotFound(err) {
						return false, nil
					}

					return false, err
				}

				observedGeneration = res.Status.ObservedGeneration
				replicas = res.Status.Replicas
				readyReplicas = res.Status.ReadyReplicas
				updatedReplicas = res.Status.UpdatedReplicas

				meta, err := json.Marshal(res.Status)
				if err != nil {
					return false, fnerrors.InternalError("failed to marshal deployment status: %w", err)
				}
				ev.ImplMetadata = meta

			case kubedef.IsGVKStatefulSet(w.GroupVersionKind):
				res, err := cli.AppsV1().StatefulSets(w.Namespace).Get(c, w.Name, metav1.GetOptions{})
				if err != nil {
					// If the resource is not visible yet, wait anyway, as the
					// only way to get here is by requesting that the resource
					// be created.
					if errors.IsNotFound(err) {
						return false, nil
					}

					return false, err
				}

				observedGeneration = res.Status.ObservedGeneration
				replicas = res.Status.Replicas
				readyReplicas = res.Status.ReadyReplicas
				updatedReplicas = res.Status.UpdatedReplicas

				meta, err := json.Marshal(res.Status)
				if err != nil {
					return false, fnerrors.InternalError("failed to marshal stateful set status: %w", err)
				}
				ev.ImplMetadata = meta

			default:
				return false, fnerrors.InternalError("%s: unsupported resource type for watching", w.GroupVersionKind)
			}

			if rs, err := fetchReplicaSetName(c, cli, w.Namespace, w.Name, w.ExpectedGen); err == nil {
				if status, err := podWaitingStatus(c, cli, w.Namespace, rs); err == nil {
					ev.WaitStatus = status
				}
			}

			ev.Ready = orchestration.Event_NOT_READY
			if observedGeneration > w.ExpectedGen {
				ev.Ready = orchestration.Event_READY
			} else if observedGeneration == w.ExpectedGen {
				if readyReplicas == replicas && updatedReplicas == replicas && replicas > 0 {
					ev.Ready = orchestration.Event_READY
				}
			}

			if ch != nil {
				ch <- ev
			}

			return ev.Ready == orchestration.Event_READY, nil
		})
	})
}
