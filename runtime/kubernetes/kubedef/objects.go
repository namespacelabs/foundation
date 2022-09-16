// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubedef

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func IsDeployment(obj runtime.Object) bool {
	return IsGVKDeployment(obj.GetObjectKind().GroupVersionKind())
}

func IsStatefulSet(obj runtime.Object) bool {
	return IsGVKStatefulSet(obj.GetObjectKind().GroupVersionKind())
}

func IsPod(obj runtime.Object) bool {
	return IsGVKPod(obj.GetObjectKind().GroupVersionKind())
}

func IsGVKDeployment(gvk schema.GroupVersionKind) bool {
	return gvk.GroupVersion().String() == "apps/v1" && gvk.Kind == "Deployment"
}

func IsGVKStatefulSet(gvk schema.GroupVersionKind) bool {
	return gvk.GroupVersion().String() == "apps/v1" && gvk.Kind == "StatefulSet"
}

func IsGVKPod(gvk schema.GroupVersionKind) bool {
	return gvk.GroupVersion().String() == "v1" && gvk.Kind == "Pod"
}
