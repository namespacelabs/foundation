// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubeparser

import (
	"bufio"
	"bytes"
	"fmt"
	"io"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"sigs.k8s.io/yaml"
)

type ParsedManifest struct {
	kubedef.Apply
	Parsed Parsed
}

func MultipleFromReader(description string, r io.Reader, parse bool) ([]ParsedManifest, error) {
	br := bufio.NewReader(r)

	var sections [][]byte
	var buf bytes.Buffer
	for {
		line, err := br.ReadBytes('\n')

		// Handle buffered data before returning errors.
		if bytes.Equal(line, []byte("---\n")) || err == io.EOF {
			copy := make([]byte, buf.Len())
			_, _ = buf.Read(copy)
			sections = append(sections, copy)
			buf.Reset()
		} else {
			buf.Write(line)
		}

		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
	}

	// For simplicity, we do a two pass parse, first we walk through all resource
	// types to instantiate the appropriate types, and then we actually parse them.

	var actuals []ParsedManifest

	for _, sec := range sections {
		p, err := Single(sec, parse)
		if err != nil {
			return nil, err
		}

		actuals = append(actuals, ParsedManifest{
			Apply: kubedef.Apply{
				SetNamespace:       true,
				Description:        fmt.Sprintf("%s: %s %s", description, p.Kind, p.Name),
				Resource:           p.Resource,
				SerializedResource: p.SerializedResource,
			},
			Parsed: p,
		})
	}

	return actuals, nil
}

type Parsed struct {
	metav1.TypeMeta
	Name               string
	Namespace          string
	Resource           runtime.Object
	SerializedResource string // JSON serialization of the resource.
}

func headerJsonOrYaml(contents []byte) (ObjHeader, error) {
	var m ObjHeader
	if err := yaml.Unmarshal(contents, &m); err != nil {
		return ObjHeader{}, err
	}

	if m.Kind == "" {
		return ObjHeader{}, fnerrors.BadInputError("kind is required")
	}
	if m.Name == "" {
		return ObjHeader{}, fnerrors.BadInputError("name is required")
	}

	return m, nil
}

func Single(contents []byte, parse bool) (Parsed, error) {
	// For simplicity, we do a two pass parse, first we walk through all resource
	// types to instantiate the appropriate types, and then we actually parse them.

	m, err := headerJsonOrYaml(contents)
	if err != nil {
		return Parsed{}, err
	}

	parsed := Parsed{
		TypeMeta:  m.TypeMeta,
		Name:      m.Name,
		Namespace: m.Namespace,
	}

	if parse {
		msg := messageTypeFromKind(m.Kind)
		if msg == nil {
			return Parsed{}, fnerrors.BadInputError("don't know how to handle %q", m.Kind)
		}

		parsed.Resource = msg
		if err := yaml.Unmarshal(contents, parsed.Resource); err != nil {
			return Parsed{}, err
		}
	} else {
		asjson, err := yaml.YAMLToJSON(contents)
		if err != nil {
			return Parsed{}, err
		}
		parsed.SerializedResource = string(asjson)
	}

	return parsed, nil
}

func messageTypeFromKind(kind string) runtime.Object {
	switch kind {
	case "Namespace":
		return &corev1.Namespace{}
	case "ServiceAccount":
		return &corev1.ServiceAccount{}
	case "ConfigMap":
		return &corev1.ConfigMap{}
	case "ClusterRole":
		return &rbacv1.ClusterRole{}
	case "ClusterRoleBinding":
		return &rbacv1.ClusterRoleBinding{}
	case "Role":
		return &rbacv1.Role{}
	case "RoleBinding":
		return &rbacv1.RoleBinding{}
	case "Service":
		return &corev1.Service{}
	case "Secret":
		return &corev1.Secret{}
	case "Deployment":
		return &appsv1.Deployment{}
	case "Ingress":
		return &networkingv1.Ingress{}
	case "IngressClass":
		return &networkingv1.IngressClass{}
	case "ValidatingWebhookConfiguration":
		return &admissionregistrationv1.ValidatingWebhookConfiguration{}
	case "CustomResourceDefinition":
		return &apiextensionsv1.CustomResourceDefinition{}
	case "Job":
		return &batchv1.Job{}
	case "Pod":
		return &corev1.Pod{}
	case "MutatingWebhookConfiguration":
		return &admissionregistrationv1.MutatingWebhookConfiguration{}
	}

	return nil
}

type ObjHeader struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

func (obj ObjHeader) GetObjectKind() schema.ObjectKind {
	return &obj.TypeMeta
}

func (obj ObjHeader) GetName() string {
	return obj.Name
}

func (obj ObjHeader) GetNamespace() string {
	return obj.Namespace
}
