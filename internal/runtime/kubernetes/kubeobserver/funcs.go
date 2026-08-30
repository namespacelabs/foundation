// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubeobserver

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func PickPod(name string) metav1.ListOptions {
	return metav1.SingleObject(metav1.ObjectMeta{Name: name})
}
