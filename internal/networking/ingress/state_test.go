// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package ingress

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestHasReadyEndpoint(t *testing.T) {
	tests := []struct {
		name      string
		endpoints *corev1.Endpoints
		want      bool
	}{
		{name: "no subsets", endpoints: &corev1.Endpoints{}},
		{
			name: "not-ready endpoint",
			endpoints: &corev1.Endpoints{Subsets: []corev1.EndpointSubset{{
				NotReadyAddresses: []corev1.EndpointAddress{{IP: "10.0.0.1"}},
			}}},
		},
		{
			name: "ready endpoint",
			endpoints: &corev1.Endpoints{Subsets: []corev1.EndpointSubset{{
				Addresses: []corev1.EndpointAddress{{IP: "10.0.0.1"}},
			}}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasReadyEndpoint(tt.endpoints); got != tt.want {
				t.Fatalf("hasReadyEndpoint() = %t, want %t", got, tt.want)
			}
		})
	}
}
