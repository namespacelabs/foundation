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
	rbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
	"namespacelabs.dev/foundation/std/resources"
)

const (
	AdminNamespace = "fn-admin"
)

type Apply struct {
	Description  string
	SetNamespace bool
	Resource     any

	InhibitEvents bool

	SchedCategory      []string
	SchedAfterCategory []string

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
	Body                any
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

type EnsureRuntimeConfig struct {
	Description          string
	RuntimeConfig        *runtimepb.RuntimeConfig
	Deployable           runtime.Deployable
	ResourceDependencies []*resources.ResourceDependency
	InjectedResources    []*OpEnsureRuntimeConfig_InjectedResource
	PersistConfiguration bool
}

type EnsureDeployment struct {
	Description             string
	Deployable              runtime.Deployable
	Resource                any
	ConfigurationVolumeName string
	SetContainerFields      []*runtimepb.SetContainerField

	RuntimeConfigDependency string

	InhibitEvents bool

	SchedCategory []string
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

func (a Apply) ToDefinitionImpl(scope ...schema.PackageName) (*schema.SerializedInvocation, *OpApply, error) {
	if a.Resource == nil {
		return nil, nil, fnerrors.InternalError("body is missing")
	}

	body, err := json.Marshal(a.Resource)
	if err != nil {
		return nil, nil, err
	}

	op := &OpApply{
		BodyJson:      string(body), // We use strings for better debuggability.
		SetNamespace:  a.SetNamespace,
		InhibitEvents: a.InhibitEvents,
	}

	if a.CheckGenerationCondition != nil {
		op.CheckGenerationCondition = &OpApply_CheckGenerationCondition{Type: a.CheckGenerationCondition.Type}
	}

	inv := &schema.SerializedInvocation{
		Description: a.Description,
		Scope:       scopeToStrings(scope),
	}

	if len(a.SchedAfterCategory) > 0 || len(a.SchedCategory) > 0 {
		inv.Order = &schema.ScheduleOrder{
			SchedCategory:      a.SchedCategory,
			SchedAfterCategory: a.SchedAfterCategory,
		}
	}

	x, err := anypb.New(op)
	if err != nil {
		return nil, nil, err
	}

	inv.Impl = x
	return inv, op, nil
}

func (a Apply) ToDefinition(scope ...schema.PackageName) (*schema.SerializedInvocation, error) {
	d, _, err := a.ToDefinitionImpl(scope...)
	return d, err
}

func (a Apply) AppliedResource() any {
	return a.Resource
}

func scopeToStrings(scope []schema.PackageName) []string {
	var r []string
	for _, s := range scope {
		if s != "" {
			r = append(r, s.String())
		}
	}
	return r
}

func (d Delete) ToDefinition(scope ...schema.PackageName) (*schema.SerializedInvocation, error) {
	x, err := anypb.New(&OpDelete{
		Resource:     d.Resource,
		Namespace:    d.Namespace,
		Name:         d.Name,
		SetNamespace: d.SetNamespace,
	})
	if err != nil {
		return nil, err
	}

	return &schema.SerializedInvocation{
		Description: d.Description,
		Impl:        x,
		Scope:       scopeToStrings(scope),
	}, nil
}

func (d DeleteList) ToDefinition(scope ...schema.PackageName) (*schema.SerializedInvocation, error) {
	x, err := anypb.New(&OpDeleteList{
		Resource:      d.Resource,
		Namespace:     d.Namespace,
		SetNamespace:  d.SetNamespace,
		LabelSelector: SerializeSelector(d.Selector),
	})
	if err != nil {
		return nil, err
	}

	return &schema.SerializedInvocation{
		Description: d.Description,
		Impl:        x,
		Scope:       scopeToStrings(scope),
	}, nil
}

func (c Create) ToDefinition(scope ...schema.PackageName) (*schema.SerializedInvocation, error) {
	if c.Body == nil {
		return nil, fnerrors.InternalError("body is missing")
	}

	body, err := json.Marshal(c.Body)
	if err != nil {
		return nil, err
	}

	x, err := anypb.New(&OpCreate{
		Resource:            c.Resource,
		SetNamespace:        c.SetNamespace,
		SkipIfAlreadyExists: c.SkipIfAlreadyExists,
		UpdateIfExisting:    c.UpdateIfExisting,
		BodyJson:            string(body), // We use strings for better debuggability.
	})
	if err != nil {
		return nil, err
	}

	return &schema.SerializedInvocation{
		Description: c.Description,
		Impl:        x,
		Scope:       scopeToStrings(scope),
	}, nil
}

func (c Create) AppliedResource() any {
	return c.Resource
}

func (ar ApplyRoleBinding) ToDefinition(scope ...schema.PackageName) (*schema.SerializedInvocation, error) {
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

	return &schema.SerializedInvocation{
		Description: ar.DescriptionBase,
		Impl:        x,
		Scope:       scopeToStrings(scope),
	}, nil
}

func (a EnsureRuntimeConfig) ToDefinition(scope ...schema.PackageName) (*schema.SerializedInvocation, error) {
	op := &OpEnsureRuntimeConfig{
		RuntimeConfig:        a.RuntimeConfig,
		Deployable:           runtime.DeployableToProto(a.Deployable),
		PersistConfiguration: a.PersistConfiguration,
		InjectResource:       a.InjectedResources,
	}

	order := &schema.ScheduleOrder{
		SchedCategory: []string{RuntimeConfigOutput(a.Deployable)},
	}

	inv := &schema.SerializedInvocation{
		Description: a.Description,
		Scope:       scopeToStrings(scope),
		Order:       order,
	}

	for _, dep := range a.ResourceDependencies {
		op.Dependency = append(op.Dependency, dep)
		order.SchedAfterCategory = append(order.SchedAfterCategory, resources.ResourceInstanceCategory(dep.ResourceInstanceId))
		inv.RequiredOutput = append(inv.RequiredOutput, dep.ResourceInstanceId)
	}

	x, err := anypb.New(op)
	if err != nil {
		return nil, err
	}

	inv.Impl = x

	return inv, nil
}

func (a EnsureRuntimeConfig) AppliedResource() any {
	return nil
}

func (a EnsureDeployment) ToDefinition(scope ...schema.PackageName) (*schema.SerializedInvocation, error) {
	if a.Resource == nil {
		return nil, fnerrors.InternalError("body is missing")
	}

	body, err := json.Marshal(a.Resource)
	if err != nil {
		return nil, err
	}

	op := &OpEnsureDeployment{
		Deployable:              runtime.DeployableToProto(a.Deployable),
		SerializedResource:      string(body), // We use strings for better debuggability.
		InhibitEvents:           a.InhibitEvents,
		ConfigurationVolumeName: a.ConfigurationVolumeName,
		SetContainerField:       a.SetContainerFields,
	}

	inv := &schema.SerializedInvocation{
		Description: a.Description,
		Scope:       scopeToStrings(scope),
	}

	inv.Order = &schema.ScheduleOrder{
		SchedCategory: a.SchedCategory,
	}

	if a.RuntimeConfigDependency != "" {
		inv.RequiredOutput = []string{a.RuntimeConfigDependency}
		inv.Order.SchedAfterCategory = append(inv.Order.SchedAfterCategory, a.RuntimeConfigDependency)
	}

	x, err := anypb.New(op)
	if err != nil {
		return nil, err
	}

	inv.Impl = x
	return inv, nil
}

func (a EnsureDeployment) AppliedResource() any {
	return a.Resource
}

func (es ExtendSpec) ToDefinition() (*schema.DefExtension, error) {
	x, err := anypb.New(es.With)
	if err != nil {
		return nil, err
	}

	return &schema.DefExtension{Impl: x}, nil
}

func (ec ExtendContainer) ToDefinition() (*schema.DefExtension, error) {
	x, err := anypb.New(ec.With)
	if err != nil {
		return nil, err
	}

	return &schema.DefExtension{Impl: x}, nil
}

func (ec ExtendInitContainer) ToDefinition() (*schema.DefExtension, error) {
	x, err := anypb.New(ec.With)
	if err != nil {
		return nil, err
	}

	return &schema.DefExtension{Impl: x}, nil
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

func RuntimeConfigOutput(deployable runtime.Deployable) string {
	return fmt.Sprintf("%s:%s", "rtconfig", deployable.GetId())
}
