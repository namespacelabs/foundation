// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubedef

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/known/anypb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	rbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	fnschema "namespacelabs.dev/foundation/schema"
)

const (
	AdminNamespace = "fn-admin"
)

type Apply struct {
	Description   string
	SetNamespace  bool
	ResourceClass *ResourceClass
	Resource      interface{}

	// If set, we wait until a status.conditions entry of matching type exists,
	// that matches the resource's generation.
	CheckGenerationCondition *CheckGenerationCondition
}

type CheckGenerationCondition struct {
	Type string
}

type Delete struct {
	Description  string
	Resource     string
	SetNamespace bool
	Namespace    string
	Name         string
}

type DeleteList struct {
	Description  string
	Resource     string
	SetNamespace bool
	Namespace    string
	Selector     map[string]string
}

type Create struct {
	Description         string
	SkipIfAlreadyExists bool
	UpdateIfExisting    bool
	SetNamespace        bool
	Resource            string
	ResourceClass       *ResourceClass
	Body                interface{}
}

type ApplyRoleBinding struct {
	DescriptionBase string
	Namespaced      bool
	RoleName        string
	RoleBindingName string
	Labels          map[string]string
	Annotations     map[string]string
	Rules           []*rbacv1.PolicyRuleApplyConfiguration
	ServiceAccount  string
}

type ExtendSpec struct {
	With *SpecExtension
}

type ExtendContainer struct {
	With *ContainerExtension
}

type ExtendInitContainer struct {
	With *InitContainerExtension
}

func (a Apply) ToDefinitionImpl(scope ...fnschema.PackageName) (*fnschema.SerializedInvocation, *OpApply, error) {
	if a.Resource == nil {
		return nil, nil, fnerrors.InternalError("body is missing")
	}

	body, err := json.Marshal(a.Resource)
	if err != nil {
		return nil, nil, err
	}

	op := &OpApply{
		BodyJson:      string(body), // We use strings for better debuggability.
		ResourceClass: a.ResourceClass,
		SetNamespace:  a.SetNamespace,
	}

	if a.CheckGenerationCondition != nil {
		op.CheckGenerationCondition = &OpApply_CheckGenerationCondition{Type: a.CheckGenerationCondition.Type}
	}

	x, err := anypb.New(op)
	if err != nil {
		return nil, nil, err
	}

	return &fnschema.SerializedInvocation{
		Description: a.Description,
		Impl:        x,
		Scope:       scopeToStrings(scope),
	}, op, nil
}

func (a Apply) ToDefinition(scope ...fnschema.PackageName) (*fnschema.SerializedInvocation, error) {
	d, _, err := a.ToDefinitionImpl(scope...)
	return d, err
}

func scopeToStrings(scope []fnschema.PackageName) []string {
	var r []string
	for _, s := range scope {
		if s != "" {
			r = append(r, s.String())
		}
	}
	return r
}

func (d Delete) ToDefinition(scope ...fnschema.PackageName) (*fnschema.SerializedInvocation, error) {
	x, err := anypb.New(&OpDelete{
		Resource:     d.Resource,
		Namespace:    d.Namespace,
		Name:         d.Name,
		SetNamespace: d.SetNamespace,
	})
	if err != nil {
		return nil, err
	}

	return &fnschema.SerializedInvocation{
		Description: d.Description,
		Impl:        x,
		Scope:       scopeToStrings(scope),
	}, nil
}

func (d DeleteList) ToDefinition(scope ...fnschema.PackageName) (*fnschema.SerializedInvocation, error) {
	x, err := anypb.New(&OpDeleteList{
		Resource:      d.Resource,
		Namespace:     d.Namespace,
		SetNamespace:  d.SetNamespace,
		LabelSelector: SerializeSelector(d.Selector),
	})
	if err != nil {
		return nil, err
	}

	return &fnschema.SerializedInvocation{
		Description: d.Description,
		Impl:        x,
		Scope:       scopeToStrings(scope),
	}, nil
}

func (c Create) ToDefinition(scope ...fnschema.PackageName) (*fnschema.SerializedInvocation, error) {
	if c.Body == nil {
		return nil, fnerrors.InternalError("body is missing")
	}

	body, err := json.Marshal(c.Body)
	if err != nil {
		return nil, err
	}

	x, err := anypb.New(&OpCreate{
		Resource:            c.Resource,
		ResourceClass:       c.ResourceClass,
		SetNamespace:        c.SetNamespace,
		SkipIfAlreadyExists: c.SkipIfAlreadyExists,
		UpdateIfExisting:    c.UpdateIfExisting,
		BodyJson:            string(body), // We use strings for better debuggability.
	})
	if err != nil {
		return nil, err
	}

	return &fnschema.SerializedInvocation{
		Description: c.Description,
		Impl:        x,
		Scope:       scopeToStrings(scope),
	}, nil
}

func (ar ApplyRoleBinding) ToDefinition(scope ...fnschema.PackageName) (*fnschema.SerializedInvocation, error) {
	body, err := json.Marshal(ar.Rules)
	if err != nil {
		return nil, err
	}

	op := &OpApplyRoleBinding{
		Namespaced:      ar.Namespaced,
		RoleName:        ar.RoleName,
		RoleBindingName: ar.RoleBindingName,
		RulesJson:       string(body),
		ServiceAccount:  ar.ServiceAccount,
	}

	for k, v := range ar.Labels {
		op.Label = append(op.Label, &OpApplyRoleBinding_KeyValue{Key: k, Value: v})
	}

	for k, v := range ar.Annotations {
		op.Annotation = append(op.Annotation, &OpApplyRoleBinding_KeyValue{Key: k, Value: v})
	}

	compare := func(a, b *OpApplyRoleBinding_KeyValue) bool {
		if a.Key == b.Key {
			return strings.Compare(a.Value, b.Value) < 0
		}
		return strings.Compare(a.Key, b.Key) < 0
	}

	slices.SortFunc(op.Label, compare)
	slices.SortFunc(op.Annotation, compare)

	x, err := anypb.New(op)
	if err != nil {
		return nil, err
	}

	return &fnschema.SerializedInvocation{
		Description: ar.DescriptionBase,
		Impl:        x,
		Scope:       scopeToStrings(scope),
	}, nil
}

func (es ExtendSpec) ToDefinition() (*fnschema.DefExtension, error) {
	x, err := anypb.New(es.With)
	if err != nil {
		return nil, err
	}

	return &fnschema.DefExtension{Impl: x}, nil
}

func (ec ExtendContainer) ToDefinition() (*fnschema.DefExtension, error) {
	x, err := anypb.New(ec.With)
	if err != nil {
		return nil, err
	}

	return &fnschema.DefExtension{Impl: x}, nil
}

func (ec ExtendInitContainer) ToDefinition() (*fnschema.DefExtension, error) {
	x, err := anypb.New(ec.With)
	if err != nil {
		return nil, err
	}

	return &fnschema.DefExtension{Impl: x}, nil
}

func SerializeSelector(selector map[string]string) string {
	var sels []string
	for k, v := range selector {
		sels = append(sels, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(sels)
	return strings.Join(sels, ",")
}

func Ego() metav1.ApplyOptions {
	return metav1.ApplyOptions{FieldManager: K8sFieldManager}
}

func (rc *ResourceClass) GroupVersion() schema.GroupVersion {
	return schema.GroupVersion{
		Group:   rc.Group,
		Version: rc.Version,
	}
}
