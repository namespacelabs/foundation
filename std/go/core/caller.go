// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package core

import (
	"context"
	"strings"

	"namespacelabs.dev/foundation/schema"
)

type ctxKey struct{}

type InstantiationPath struct {
	path []schema.PackageName
}

func (ip *InstantiationPath) Last() schema.PackageName {
	if len(ip.path) == 0 {
		return ""
	}
	return ip.path[len(ip.path)-1]
}

func (ip *InstantiationPath) String() string {
	var a []string
	for _, step := range ip.path {
		a = append(a, step.String())
	}
	return strings.Join(a, ",")
}

// PathFromContext returns the InstantiationPath associated with the ctx.
// If no logger is associated, nil is returned
func PathFromContext(ctx context.Context) *InstantiationPath {
	v := ctx.Value(ctxKey{})
	if v == nil {
		return nil
	}
	return v.(*InstantiationPath)
}

// WithContext returns a copy of ctx with ip associated. If an instance of InstantiationPath
// is already in the context, the value is overwritten.
func (ip *InstantiationPath) WithContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKey{}, ip)
}

func (ip *InstantiationPath) Append(pkg schema.PackageName) *InstantiationPath {
	if ip == nil {
		return &InstantiationPath{path: []schema.PackageName{pkg}}
	}

	copy := *ip
	copy.path = append(copy.path, pkg)
	return &copy
}

func (ip *InstantiationPath) trim() *InstantiationPath {
	return &InstantiationPath{path: []schema.PackageName{ip.Last()}}
}
