// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubedef

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Object interface {
	GroupVersionKind() schema.GroupVersionKind
	GetName() string
	GetNamespace() string
	GetLabels() map[string]string
}

func IsDeployment(obj Object) bool {
	return IsGVKDeployment(obj.GroupVersionKind())
}

func IsStatefulSet(obj Object) bool {
	return IsGVKStatefulSet(obj.GroupVersionKind())
}

func IsDaemonSetSet(obj Object) bool {
	return IsGVKDaemonSet(obj.GroupVersionKind())
}

func IsPod(obj Object) bool {
	return IsGVKPod(obj.GroupVersionKind())
}

func IsService(obj Object) bool {
	return IsGVKService(obj.GroupVersionKind())
}

func IsGVKDeployment(gvk schema.GroupVersionKind) bool {
	return gvk.GroupVersion().String() == "apps/v1" && gvk.Kind == "Deployment"
}

func IsGVKStatefulSet(gvk schema.GroupVersionKind) bool {
	return gvk.GroupVersion().String() == "apps/v1" && gvk.Kind == "StatefulSet"
}

func IsGVKDaemonSet(gvk schema.GroupVersionKind) bool {
	return gvk.GroupVersion().String() == "apps/v1" && gvk.Kind == "DaemonSet"
}

func IsGVKPod(gvk schema.GroupVersionKind) bool {
	return gvk.GroupVersion().String() == "v1" && gvk.Kind == "Pod"
}

func IsGVKService(gvk schema.GroupVersionKind) bool {
	return gvk.GroupVersion().String() == "v1" && gvk.Kind == "Service"
}

func IsCRD(obj Object) bool {
	return IsGVKCRD(obj.GroupVersionKind())
}

func IsGVKCRD(gvk schema.GroupVersionKind) bool {
	return gvk.GroupVersion().String() == "apiextensions.k8s.io/v1" && gvk.Kind == "CustomResourceDefinition"
}
