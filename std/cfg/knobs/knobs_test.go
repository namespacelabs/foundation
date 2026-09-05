// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package knobs

import (
	"testing"

	"namespacelabs.dev/foundation/std/cfg"
)

func TestSetOverridesPreviousValue(t *testing.T) {
	knob := Bool("test_set_override", "", false)
	configuration := cfg.MakeConfigurationWith("test", nil, cfg.ConfigurationSlice{})
	configuration = knob.Set(configuration, true)
	configuration = knob.Set(configuration, false)

	if knob.Get(configuration) {
		t.Fatal("Set did not override the previous knob value")
	}
}
