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
	admissionregistrationv1 "k8s.io/client-go/applyconfigurations/admissionregistration/v1"
	appsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	batchv1 "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
	v1 "k8s.io/client-go/applyconfigurations/meta/v1"
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
			Name:        p.Name,
			Namespace:   p.Namespace,
			Resource:    p.Resource,
			Body:        p.Body,
		})
	}

	return actuals, nil
}

type Parsed struct {
	Kind      string
	Name      string
	Namespace string
	Resource  string
	Body      interface{}
}

func Single(contents []byte) (Parsed, error) {
	// For simplicity, we do a two pass parse, first we walk through all resource
	// types to instantiate the appropriate types, and then we actually parse them.

	var m objHeader
	if err := yaml.Unmarshal(contents, &m); err != nil {
		return Parsed{}, err
	}

	if m.Kind == nil {
		return Parsed{}, fnerrors.BadInputError("kind is required")
	}
	if m.Name == nil {
		return Parsed{}, fnerrors.BadInputError("name is required")
	}

	msg, typ := msgFromKind(*m.Kind)
	name := *m.Name

	if msg == nil {
		return Parsed{}, fnerrors.BadInputError("don't know how to handle %q", *m.Kind)
	}

	var ns string
	if m.Namespace != nil {
		ns = *m.Namespace
	}

	parsed := Parsed{
		Kind:      *m.Kind,
		Name:      name,
		Namespace: ns,
		Resource:  typ,
		Body:      msg,
	}

	if err := yaml.Unmarshal(contents, parsed.Body); err != nil {
		return Parsed{}, err
	}

	return parsed, nil
}

func msgFromKind(kind string) (interface{}, string) {
	switch kind {
	case "Namespace":
		return &corev1.NamespaceApplyConfiguration{}, "namespaces"
	case "ServiceAccount":
		return &corev1.ServiceAccountApplyConfiguration{}, "serviceaccounts"
	case "ConfigMap":
		return &corev1.ConfigMapApplyConfiguration{}, "configmaps"
	case "ClusterRole":
		return &rbacv1.ClusterRoleApplyConfiguration{}, "clusterroles"
	case "ClusterRoleBinding":
		return &rbacv1.ClusterRoleBindingApplyConfiguration{}, "clusterrolebindings"
	case "Role":
		return &rbacv1.RoleApplyConfiguration{}, "roles"
	case "RoleBinding":
		return &rbacv1.RoleBindingApplyConfiguration{}, "rolebindings"
	case "Service":
		return &corev1.ServiceApplyConfiguration{}, "services"
	case "Deployment":
		return &appsv1.DeploymentApplyConfiguration{}, "deployments"
	case "IngressClass":
		return &networkingv1.IngressClassApplyConfiguration{}, "ingressclasses"
	case "ValidatingWebhookConfiguration":
		return &admissionregistrationv1.ValidatingWebhookConfigurationApplyConfiguration{}, "validatingwebhookconfigurations"
	case "CustomResourceDefinition":
		return &apiextensionsv1.CustomResourceDefinition{}, "customresourcedefinitions"
	case "Job":
		return &batchv1.JobApplyConfiguration{}, "jobs"
	}

	return nil, ""
}

type objHeader struct {
	v1.TypeMetaApplyConfiguration    `json:",inline"`
	*v1.ObjectMetaApplyConfiguration `json:"metadata,omitempty"`
}
