// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubedef

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"google.golang.org/protobuf/types/known/anypb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"namespacelabs.dev/foundation/internal/fnerrors"
	fnschema "namespacelabs.dev/foundation/schema"
)

const (
	AdminNamespace = "fn-admin"
)

type Apply struct {
	Description   string
	ResourceClass *ResourceClass
	Resource      interface{}
}

type Delete struct {
	Description string
	Resource    string
	Namespace   string
	Name        string
}

type DeleteList struct {
	Description string
	Resource    string
	Namespace   string
	Selector    map[string]string
}

type Create struct {
	Description         string
	SkipIfAlreadyExists bool
	UpdateIfExisting    bool
	Resource            string
	ResourceClass       *ResourceClass
	Body                interface{}
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

func (a Apply) ToDefinition(scope ...fnschema.PackageName) (*fnschema.SerializedInvocation, error) {
	if a.Resource == nil {
		return nil, fnerrors.InternalError("body is missing")
	}

	body, err := json.Marshal(a.Resource)
	if err != nil {
		return nil, err
	}

	x, err := anypb.New(&OpApply{
		BodyJson:      string(body), // We use strings for better debuggability.
		ResourceClass: a.ResourceClass,
	})
	if err != nil {
		return nil, err
	}

	return &fnschema.SerializedInvocation{
		Description: a.Description,
		Impl:        x,
		Scope:       scopeToStrings(scope),
	}, nil
}

func scopeToStrings(scope []fnschema.PackageName) []string {
	r := make([]string, len(scope))
	for k, s := range scope {
		r[k] = s.String()
	}
	return r
}

func (d Delete) ToDefinition(scope ...fnschema.PackageName) (*fnschema.SerializedInvocation, error) {
	x, err := anypb.New(&OpDelete{
		Resource:  d.Resource,
		Namespace: d.Namespace,
		Name:      d.Name,
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
