// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubedef

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
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

func PlanOrder(gvk schema.GroupVersionKind) *fnschema.ScheduleOrder {
	var cats, after []string

	// Ignore versions in ordering. They don't play much role.
	cats = append(cats, MakeSchedCat(gvk.GroupKind()))

	// All objects are created after namespaces, unless they're a namespace.
	if !(gvk.GroupVersion().String() == "v1" && gvk.Kind == "Namespace") {
		after = append(after, MakeSchedCat(schema.GroupKind{Kind: "Namespace"}))

		// This is not strictly necessary but simplifies the rules below.
		if !(gvk.GroupVersion().String() == "v1" && gvk.Kind == "ServiceAccount") {
			after = append(after, MakeSchedCat(schema.GroupKind{Kind: "ServiceAccount"}))
		}
	}

	switch gvk.GroupVersion().String() {
	case "v1":
		switch gvk.Kind {
		case "Pod":
			after = append(after, Sched_ResourceLike...)

		case "Service":
			after = append(after, Sched_JobLike...)
		}

	case "apiextensions.k8s.io/v1":
		// Nothing to do but don't trigger default.

	case "rbac.authorization.k8s.io/v1":
		// Nothing to do but don't trigger default.

	case "apps/v1":
		after = append(after, Sched_ResourceLike...)

	case "networking.k8s.io/v1":
		after = append(after, MakeSchedCat(schema.GroupKind{Kind: "Service"}))

	case "batch/v1":
		after = append(after, Sched_ResourceLike...)

	default:
		// If we don't know the group, assume it's deployed after jobs (e.g. CRDs).
		after = append(after, Sched_JobLike...)
	}

	return &fnschema.ScheduleOrder{SchedCategory: cats, SchedAfterCategory: after}
}

func MakeSchedCat(gv schema.GroupKind) string {
	return fmt.Sprintf("k8s:gv:%s:%s", gv.Group, gv.Kind)
}
