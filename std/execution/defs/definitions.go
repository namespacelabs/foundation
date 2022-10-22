// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package defs

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/schema"
)

type DefList struct {
	descriptions []string
	impls        []proto.Message
	scopes       []schema.PackageList
}

func (d *DefList) Add(description string, impl proto.Message, scope ...schema.PackageName) {
	d.descriptions = append(d.descriptions, description)
	d.impls = append(d.impls, impl)

	var sl schema.PackageList
	sl.AddMultiple(scope...)
	d.scopes = append(d.scopes, sl)
}

func (d *DefList) Serialize() ([]*schema.SerializedInvocation, error) {
	var defs []*schema.SerializedInvocation
	for k := range d.descriptions {
		serialized, err := anypb.New(d.impls[k])
		if err != nil {
			return nil, err
		}
		defs = append(defs, &schema.SerializedInvocation{
			Description: d.descriptions[k],
			Impl:        serialized,
			Scope:       d.scopes[k].PackageNamesAsString(),
		})
	}
	return defs, nil
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
