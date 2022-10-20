// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeobserver

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func PickPod(name string) metav1.ListOptions {
	return metav1.SingleObject(metav1.ObjectMeta{Name: name})
}
