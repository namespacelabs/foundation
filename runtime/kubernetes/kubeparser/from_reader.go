// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeparser

import (
	"bufio"
	"bytes"
	"fmt"
	"io"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	admissionregistrationv1 "k8s.io/client-go/applyconfigurations/admissionregistration/v1"
	appsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	batchv1 "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
	networkingv1 "k8s.io/client-go/applyconfigurations/networking/v1"
	rbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"sigs.k8s.io/yaml"
)

func MultipleFromReader(description string, r io.Reader) ([]kubedef.Apply, error) {
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

	var actuals []kubedef.Apply

	for _, sec := range sections {
		p, err := Single(sec)
		if err != nil {
			return nil, err
		}

		actuals = append(actuals, kubedef.Apply{
			Description: fmt.Sprintf("%s: %s %s", description, p.Kind, p.Name),
			Resource:    p.Resource,
		})
	}

	return actuals, nil
}

type Parsed struct {
	Kind      string
	Name      string
	Namespace string
	Resource  interface{}
}

func Header(contents []byte) (ObjHeader, error) {
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

func Single(contents []byte) (Parsed, error) {
	// For simplicity, we do a two pass parse, first we walk through all resource
	// types to instantiate the appropriate types, and then we actually parse them.

	m, err := Header(contents)
	if err != nil {
		return Parsed{}, err
	}

	msg := MessageTypeFromKind(m.Kind)

	if msg == nil {
		return Parsed{}, fnerrors.BadInputError("don't know how to handle %q", m.Kind)
	}

	parsed := Parsed{
		Kind:      m.Kind,
		Name:      m.Name,
		Namespace: m.Namespace,
		Resource:  msg,
	}

	if err := yaml.Unmarshal(contents, parsed.Resource); err != nil {
		return Parsed{}, err
	}

	return parsed, nil
}

func ResourceEndpointFromKind(kind string) string {
	switch kind {
	case "Namespace":
		return "namespaces"
	case "ServiceAccount":
		return "serviceaccounts"
	case "ConfigMap":
		return "configmaps"
	case "ClusterRole":
		return "clusterroles"
	case "ClusterRoleBinding":
		return "clusterrolebindings"
	case "Role":
		return "roles"
	case "RoleBinding":
		return "rolebindings"
	case "Pod":
		return "pods"
	case "Service":
		return "services"
	case "Secret":
		return "secrets"
	case "Deployment":
		return "deployments"
	case "StatefulSet":
		return "statefulsets"
	case "Ingress":
		return "ingresses"
	case "IngressClass":
		return "ingressclasses"
	case "ValidatingWebhookConfiguration":
		return "validatingwebhookconfigurations"
	case "CustomResourceDefinition":
		return "customresourcedefinitions"
	case "Job":
		return "jobs"
	case "PersistentVolumeClaim":
		return "persistentvolumeclaims"
	}

	return ""
}

func MessageTypeFromKind(kind string) interface{} {
	switch kind {
	case "Namespace":
		return &corev1.NamespaceApplyConfiguration{}
	case "ServiceAccount":
		return &corev1.ServiceAccountApplyConfiguration{}
	case "ConfigMap":
		return &corev1.ConfigMapApplyConfiguration{}
	case "ClusterRole":
		return &rbacv1.ClusterRoleApplyConfiguration{}
	case "ClusterRoleBinding":
		return &rbacv1.ClusterRoleBindingApplyConfiguration{}
	case "Role":
		return &rbacv1.RoleApplyConfiguration{}
	case "RoleBinding":
		return &rbacv1.RoleBindingApplyConfiguration{}
	case "Service":
		return &corev1.ServiceApplyConfiguration{}
	case "Secret":
		return &corev1.SecretApplyConfiguration{}
	case "Deployment":
		return &appsv1.DeploymentApplyConfiguration{}
	case "Ingress":
		return &networkingv1.IngressApplyConfiguration{}
	case "IngressClass":
		return &networkingv1.IngressClassApplyConfiguration{}
	case "ValidatingWebhookConfiguration":
		return &admissionregistrationv1.ValidatingWebhookConfigurationApplyConfiguration{}
	case "CustomResourceDefinition":
		return &apiextensionsv1.CustomResourceDefinition{}
	case "Job":
		return &batchv1.JobApplyConfiguration{}
	}

	return nil
}

func ResourceFromKind(kind string) (interface{}, string) {
	return MessageTypeFromKind(kind), ResourceEndpointFromKind(kind)
}

type ObjHeader struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

func (obj ObjHeader) GetObjectKind() schema.ObjectKind {
	return &obj.TypeMeta
}
