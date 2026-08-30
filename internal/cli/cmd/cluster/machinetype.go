// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"strconv"
	"strings"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

// ParseMachineType parses a machine type string in the format "CPUxMemoryGB" (e.g., "4x8")
// and returns the vCPU count and memory in megabytes.
// This function is used for converting the shorthand format into structured data.
func ParseMachineType(machineType string) (vcpu int32, memoryMB int32, err error) {
	parts := strings.Split(machineType, "x")
	if len(parts) != 2 {
		return 0, 0, fnerrors.Newf("invalid machine_type format: expected 'CPUxMemoryGB' (e.g., '4x8'), got %q", machineType)
	}

	cpu, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil {
		return 0, 0, fnerrors.Newf("invalid CPU value in machine_type %q: %w", machineType, err)
	}

	memoryGB, err := strconv.ParseInt(parts[1], 10, 32)
	if err != nil {
		return 0, 0, fnerrors.Newf("invalid memory value in machine_type %q: %w", machineType, err)
	}

	return int32(cpu), int32(memoryGB * 1024), nil
}
