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
	applyrbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/types"
)

type Apply struct {
	Description string
	Resource    string // XXX this can be implied from `kind` in the body. See #339.
	Namespace   string // XXX this can be implied from `namespace` in the body. See #339.
	Name        string // XXX this can be implied from `name` in the body. See #339.
	Body        interface{}
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
	Description string
	IfMissing   bool
	Resource    string // XXX this can be implied from `kind` in the body. See #339.
	Namespace   string // XXX this can be implied from `namespace` in the body. See #339.
	Name        string // XXX this can be implied from `name` in the body. See #339.
	Body        interface{}
}

// This is a temporary type; usage should be limited. It's a workaround until we
// can compose invocations, so secrets can wrap a "create secret payload" invocation
// around the user-provided invocation.
type CreateSecretConditionally struct {
	Description       string
	Namespace         string
	Name              string
	UserSpecifiedName string
	Invocation        *types.DeferredInvocation
}

// Only a limited set of nodes is allowed to set this.
type Admin struct {
	Description string
	Name        string
	Rules       []*applyrbacv1.PolicyRuleApplyConfiguration
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

func (a Apply) ToDefinition(scope ...schema.PackageName) (*schema.Definition, error) {
	if a.Body == nil {
		return nil, fnerrors.InternalError("body is missing")
	}

	body, err := json.Marshal(a.Body)
	if err != nil {
		return nil, err
	}

	x, err := anypb.New(&OpApply{
		Resource:  a.Resource,
		Namespace: a.Namespace,
		Name:      a.Name,
		BodyJson:  string(body), // We use strings for better debuggability.
	})
	if err != nil {
		return nil, err
	}

	return &schema.Definition{
		Description: a.Description,
		Impl:        x,
		Scope:       scopeToStrings(scope),
	}, nil
}

func scopeToStrings(scope []schema.PackageName) []string {
	r := make([]string, len(scope))
	for k, s := range scope {
		r[k] = s.String()
	}
	return r
}

func (d Delete) ToDefinition(scope ...schema.PackageName) (*schema.Definition, error) {
	x, err := anypb.New(&OpDelete{
		Resource:  d.Resource,
		Namespace: d.Namespace,
		Name:      d.Name,
	})
	if err != nil {
		return nil, err
	}

	return &schema.Definition{
		Description: d.Description,
		Impl:        x,
		Scope:       scopeToStrings(scope),
	}, nil
}

func (d DeleteList) ToDefinition(scope ...schema.PackageName) (*schema.Definition, error) {
	x, err := anypb.New(&OpDeleteList{
		Resource:      d.Resource,
		Namespace:     d.Namespace,
		LabelSelector: SerializeSelector(d.Selector),
	})
	if err != nil {
		return nil, err
	}

	return &schema.Definition{
		Description: d.Description,
		Impl:        x,
		Scope:       scopeToStrings(scope),
	}, nil
}

func (c Create) ToDefinition(scope ...schema.PackageName) (*schema.Definition, error) {
	if c.Body == nil {
		return nil, fnerrors.InternalError("body is missing")
	}

	body, err := json.Marshal(c.Body)
	if err != nil {
		return nil, err
	}

	x, err := anypb.New(&OpCreate{
		Resource:  c.Resource,
		IfMissing: c.IfMissing,
		Namespace: c.Namespace,
		Name:      c.Name,
		BodyJson:  string(body), // We use strings for better debuggability.
	})
	if err != nil {
		return nil, err
	}

	return &schema.Definition{
		Description: c.Description,
		Impl:        x,
		Scope:       scopeToStrings(scope),
	}, nil
}

func (c CreateSecretConditionally) ToDefinition(scope ...schema.PackageName) (*schema.Definition, error) {
	if c.Invocation == nil {
		return nil, fnerrors.InternalError("invocation is missing")
	}

	x, err := anypb.New(&OpCreateSecretConditionally{
		Namespace:         c.Namespace,
		Name:              c.Name,
		UserSpecifiedName: c.UserSpecifiedName,
		Invocation:        c.Invocation,
	})
	if err != nil {
		return nil, err
	}

	return &schema.Definition{
		Description: c.Description,
		Impl:        x,
		Scope:       scopeToStrings(scope),
	}, nil
}

func (a Admin) ToDefinition(scope ...schema.PackageName) (*schema.Definition, error) {
	if len(a.Rules) == 0 {
		return nil, fnerrors.InternalError("no admin rules specified")
	}

	rules, err := json.Marshal(a.Rules)
	if err != nil {
		return nil, err
	}

	x, err := anypb.New(&OpAdmin{
		Name:      a.Name,
		RulesJson: string(rules),
	})
	if err != nil {
		return nil, err
	}

	return &schema.Definition{
		Description: a.Description,
		Impl:        x,
		Scope:       scopeToStrings(scope),
	}, nil
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
