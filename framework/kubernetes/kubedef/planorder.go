// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubedef

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"namespacelabs.dev/foundation/internal/runtime"
	fnschema "namespacelabs.dev/foundation/schema"
)

var (
	Sched_JobLike = []string{
		MakeSchedCat(schema.GroupKind{Kind: "Pod"}),
		MakeSchedCat(schema.GroupKind{Group: "apps", Kind: "Deployment"}),
		MakeSchedCat(schema.GroupKind{Group: "apps", Kind: "StatefulSet"}),
	}

	Sched_ResourceLike = []string{
		MakeSchedCat(schema.GroupKind{Kind: "ConfigMap"}),
		MakeSchedCat(schema.GroupKind{Kind: "Secret"}),
		MakeSchedCat(schema.GroupKind{Kind: "PersistentVolumeClaim"}),
	}
)

func PlanOrder(gvk schema.GroupVersionKind, namespace, name string) *fnschema.ScheduleOrder {
	var cats, after []string

	// Ignore versions in ordering. They don't play much role.
	cats = append(cats, MakeSchedCat(gvk.GroupKind()))

	if gvk.GroupVersion().String() == "v1" && gvk.Kind == "Namespace" {
		cats = append(cats, MakeNamespaceCat(name))
	} else if namespace != "" {
		after = append(after, MakeNamespaceCat(namespace))
	}

	switch gvk.GroupVersion().String() {
	case "v1":
		switch gvk.Kind {
		case "Pod":
			after = append(after, MakeSchedCat(schema.GroupKind{Kind: "ServiceAccount"}))
			after = append(after, Sched_ResourceLike...)
		}

	case "apiextensions.k8s.io/v1":
		// Nothing to do but don't trigger default.

	case "rbac.authorization.k8s.io/v1":
		// Nothing to do but don't trigger default.

	case "apps/v1":
		after = append(after, Sched_ResourceLike...)

	case "networking.k8s.io/v1":
		// Deploy ingress (and network resources) after services and secrets.
		after = append(after, MakeSchedCat(schema.GroupKind{Kind: "Service"}), MakeSchedCat(schema.GroupKind{Kind: "Secret"}))

	case "batch/v1":
		after = append(after, Sched_ResourceLike...)

	default:
		// If we don't know the group, assume it's deployed after jobs (e.g. CRDs).
		after = append(after, Sched_JobLike...)
	}

	return &fnschema.ScheduleOrder{SchedCategory: cats, SchedAfterCategory: after}
}

func MakeNamespaceCat(namespace string) string {
	return fmt.Sprintf("kube:namespace:%s", namespace)
}

func MakeServicesCat(deployable runtime.Deployable) string {
	return fmt.Sprintf("kube:services:%s", deployable.GetId())
}

func MakeSchedCat(gv schema.GroupKind) string {
	return fmt.Sprintf("kube:gv:%s:%s", gv.Group, gv.Kind)
}
