// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkit

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

type JobDefinition struct {
	Namespace      *corev1.Namespace
	StatefulSet    *appsv1.StatefulSet
	Service        *corev1.Service
	MatchingLabels map[string]string
}

func JobDef() *JobDefinition {
	nsbuild := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns-build",
		},
	}

	replicas := int32(1)

	labels := map[string]string{"app": "buildkitd"}

	buildkitd := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: nsbuild.Name,
			Name:      "buildkitd",
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: "buildkitd",
			Replicas:    &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "buildkitd",
							Image: "moby/buildkit:v0.11.0",
							Args: []string{
								"--addr", "unix:///run/buildkit/buildkitd.sock",
								"--addr", "tcp://0.0.0.0:10000",
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: pointer.Bool(true),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "var-lib-buildkit",
									MountPath: "/var/lib/buildkit",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "var-lib-buildkit",
							// Emptydir.
						},
					},
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{{
									MatchExpressions: []corev1.NodeSelectorRequirement{{
										Key:      "app.kubernetes.io/managed-by",
										Operator: "In",
										Values:   []string{"linux/amd64", "linux/arm64"},
									}},
								}},
							},
						},
					},
				},
			},
		},
	}

	buildkitService := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "buildkitd",
			Namespace: nsbuild.Name,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{{
				Port:     10000,
				Protocol: corev1.ProtocolTCP,
			}},
			Selector: labels,
		},
	}

	return &JobDefinition{
		Namespace:      nsbuild,
		StatefulSet:    buildkitd,
		Service:        buildkitService,
		MatchingLabels: labels,
	}
}
