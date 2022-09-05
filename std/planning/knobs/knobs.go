// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package knobs

import (
	"fmt"
	"reflect"

	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/std/planning/knobs/config"
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

func Define[V any](name, description string, value Value) Knob[V] {
	knob := Knob[V]{name, description, value}
	knobs = append(knobs, knob)
	return knob
}

func (knob Knob[V]) Get(cfg planning.Configuration) V {
	knobs, err := planning.GetMultiple[*config.Knob](cfg)
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
	case reflect.Bool:
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
