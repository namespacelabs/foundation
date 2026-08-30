// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package defs

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/schema"
)

type Transformer func(*schema.SerializedInvocation)

type DefList struct {
	descriptions []string
	impls        []proto.Message
	transformers [][]Transformer
}

func (d *DefList) Add(description string, impl proto.Message, scope ...schema.PackageName) {
	var sl schema.PackageList
	sl.AddMultiple(scope...)

	d.AddExt(description, impl, func(di *schema.SerializedInvocation) {
		di.Scope = sl.PackageNamesAsString()
	})
}

func (d *DefList) AddExt(description string, impl proto.Message, transformers ...Transformer) {
	d.descriptions = append(d.descriptions, description)
	d.impls = append(d.impls, impl)
	d.transformers = append(d.transformers, transformers)
}

func (d *DefList) AddProgram(p DefList) {
	d.descriptions = append(d.descriptions, p.descriptions...)
	d.impls = append(d.impls, p.impls...)
	d.transformers = append(d.transformers, p.transformers...)
}

func (d *DefList) Serialize() ([]*schema.SerializedInvocation, error) {
	var defs []*schema.SerializedInvocation
	for k := range d.descriptions {
		serialized, err := anypb.New(d.impls[k])
		if err != nil {
			return nil, err
		}
		di := &schema.SerializedInvocation{
			Description: d.descriptions[k],
			Impl:        serialized,
		}
		for _, transformer := range d.transformers[k] {
			transformer(di)
		}
		defs = append(defs, di)
	}
	return defs, nil
}

func Category(names ...string) Transformer {
	return func(di *schema.SerializedInvocation) {
		if di.Order == nil {
			di.Order = &schema.ScheduleOrder{
				SchedCategory: names,
			}
		} else {
			di.Order.SchedCategory = append(di.Order.SchedCategory, names...)
		}
	}
}

func DependsOn(names ...string) Transformer {
	return func(di *schema.SerializedInvocation) {
		if di.Order == nil {
			di.Order = &schema.ScheduleOrder{
				SchedAfterCategory: names,
			}
		} else {
			di.Order.SchedAfterCategory = append(di.Order.SchedAfterCategory, names...)
		}
	}
}

func Consumes(name string) Transformer {
	return func(di *schema.SerializedInvocation) {
		di.RequiredOutput = append(di.RequiredOutput, name)
	}
}

func MinimumVersion(version int32) Transformer {
	return func(di *schema.SerializedInvocation) {
		di.MinimumVersion = version
	}
}

func Make[V MakeDefinition](ops ...V) ([]*schema.SerializedInvocation, error) {
	var defs []*schema.SerializedInvocation
	for _, m := range ops {
		def, err := m.ToDefinition()
		if err != nil {
			return nil, err
		}

		defs = append(defs, def)
	}

	return defs, nil
}

func Static(description string, impl proto.Message) MakeDefinition {
	return staticDef{description, impl}
}

type staticDef struct {
	description string
	impl        proto.Message
}

func (def staticDef) ToDefinition(scope ...schema.PackageName) (*schema.SerializedInvocation, error) {
	serialized, err := anypb.New(def.impl)
	if err != nil {
		return nil, err
	}

	return &schema.SerializedInvocation{
		Description: def.description,
		Impl:        serialized,
		Scope:       schema.Strs(scope...),
	}, nil
}
