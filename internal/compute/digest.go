// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package compute

import (
	"context"
	"reflect"

	"namespacelabs.dev/foundation/schema"
)

type ValueDigester[V any] interface {
	ComputeDigest(context.Context, V) (schema.Digest, error)
}

type ComputeDigestFunc func(context.Context, any) (schema.Digest, error)

type registeredDigester struct {
	typ    reflect.Type
	digest ComputeDigestFunc
}

var digesters []registeredDigester

func RegisterDigester[V any](d ValueDigester[V]) {
	var t *V
	digesters = append(digesters, registeredDigester{
		typ: interfaceType(t),
		digest: func(ctx context.Context, value any) (schema.Digest, error) {
			return d.ComputeDigest(ctx, value.(V))
		},
	})
}

func interfaceType(t any) reflect.Type {
	vt := reflect.TypeOf(t)
	if vt.Kind() != reflect.Ptr {
		panic("expected pointer to type")
	}

	elem := vt.Elem()
	switch elem.Kind() {
	case reflect.Interface, reflect.Slice:
		return elem
	}

	panic("unexpected type, got " + elem.String())
}

func digesterFor(outputType any) ComputeDigestFunc {
	vt := reflect.TypeOf(outputType)
	if vt == nil || vt.Kind() != reflect.Ptr {
		return nil
	}

	elem := vt.Elem()
	for _, d := range digesters {
		if elem == d.typ || (d.typ.Kind() == reflect.Interface && (vt.Implements(d.typ) || elem.Implements(d.typ))) {
			return d.digest
		}
	}

	return nil
}
