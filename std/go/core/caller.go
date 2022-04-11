// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package core

import (
	"context"
	"fmt"
	"strings"

	"namespacelabs.dev/foundation/schema"
)

type ctxKey struct{}

type instantiation struct {
	PackageName schema.PackageName
	Instance    string
}

type InstantiationPath struct {
	path []instantiation
}

func (ip *InstantiationPath) Last() schema.PackageName {
	if len(ip.path) == 0 {
		return ""
	}
	return ip.path[len(ip.path)-1].PackageName
}

func (ip *InstantiationPath) String() string {
	var inst []string
	for _, step := range ip.path {
		inst = append(inst, fmt.Sprintf("%s:%s", step.PackageName, step.Instance))
	}
	return strings.Join(inst, "->")
}

// PathFromContext returns the InstantiationPath associated with the ctx.
// If no logger is associated, nil is returned
func PathFromContext(ctx context.Context) *InstantiationPath {
	return ctx.Value(ctxKey{}).(*InstantiationPath)
}

// WithContext returns a copy of ctx with ip associated. If an instance of InstantiationPath
// is already in the context, the value is overwritten.
func (ip *InstantiationPath) WithContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKey{}, ip)
}

func resetInstantiationPath(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKey{}, &InstantiationPath{})
}

func (ip *InstantiationPath) Append(pkg schema.PackageName, inst string) *InstantiationPath {
	last := instantiation{
		PackageName: pkg,
		Instance:    inst,
	}

	if ip == nil {
		return &InstantiationPath{path: []instantiation{last}}
	}

	copy := *ip
	copy.path = append(copy.path, last)
	return &copy
}
