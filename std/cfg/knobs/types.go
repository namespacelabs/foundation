// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package knobs

import (
	"github.com/spf13/pflag"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type Value interface {
	get() any
	decode(*anypb.Any) (any, error)
	setupFlags(flags *pflag.FlagSet, name, description string)
}

func BoolValue[V bool](defaultValue bool) Value {
	return &boolv{value: defaultValue}
}

type boolv struct {
	value bool
}

func (b *boolv) get() any {
	return b.value
}

func (b *boolv) decode(any *anypb.Any) (any, error) {
	bv := &wrapperspb.BoolValue{}
	if err := any.UnmarshalTo(bv); err != nil {
		return nil, err
	}
	return bv.Value, nil
}

func (b *boolv) setupFlags(flags *pflag.FlagSet, name, description string) {
	flags.BoolVar(&b.value, name, b.value, description)
}

func StringValue[V string](defaultValue string) Value {
	return &stringv{value: defaultValue}
}

type stringv struct {
	value string
}

func (b *stringv) get() any {
	return b.value
}

func (b *stringv) decode(any *anypb.Any) (any, error) {
	bv := &wrapperspb.StringValue{}
	if err := any.UnmarshalTo(bv); err != nil {
		return nil, err
	}
	return bv.Value, nil
}

func (b *stringv) setupFlags(flags *pflag.FlagSet, name, description string) {
	flags.StringVar(&b.value, name, b.value, description)
}
