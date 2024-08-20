// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package knobs

import (
	"fmt"
	"reflect"

	"github.com/spf13/pflag"

	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/cfg/knobs/config"
)

var knobs []internalKnob

type Knob[V any] struct {
	name        string
	description string
	value       Value
}

type internalKnob interface {
	setupFlags(*pflag.FlagSet)
}

func Bool(name, description string, value bool) Knob[bool] {
	return Define[bool](name, description, BoolValue(value))
}

func String(name, description string, value string) Knob[string] {
	return Define[string](name, description, StringValue(value))
}

func Define[V any](name, description string, value Value) Knob[V] {
	knob := Knob[V]{name, description, value}
	knobs = append(knobs, knob)
	return knob
}

func (knob Knob[V]) Get(src cfg.Configuration) V {
	knobs, err := cfg.GetMultiple[*config.Knob](src)
	if err != nil {
		// For this to fail, config.Knob would have to not been compiled into the binary?
		panic(err)
	}

	for _, k := range knobs {
		if k.Name == knob.name {
			raw, err := knob.value.decode(k.Value)
			if err != nil {
				panic(err)
			}
			return raw.(V)
		}
	}

	return knob.value.get().(V)
}

func (knob Knob[V]) setupFlags(flags *pflag.FlagSet) {
	var v V
	switch reflect.TypeOf(v).Kind() {
	case reflect.Bool, reflect.String:
		knob.value.setupFlags(flags, knob.name, knob.description)

	default:
		panic(fmt.Sprintf("%s unsupported", reflect.TypeOf(v).Kind()))
	}

	_ = flags.MarkHidden(knob.name)
}

func SetupFlags(flags *pflag.FlagSet) {
	for _, knob := range knobs {
		knob.setupFlags(flags)
	}
}
