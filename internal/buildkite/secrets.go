// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkite

import (
	"fmt"
	"os/exec"
	"strings"
)

func RedactBuildkiteSecretValue(value string) error {
	buildkiteAgent, err := exec.LookPath("buildkite-agent")
	if err != nil {
		return fmt.Errorf("failed to find buildkite-agent: %w", err)
	}

	redactor := exec.Command(buildkiteAgent, "redactor", "add")
	redactor.Stdin = strings.NewReader(value)
	if err := redactor.Run(); err != nil {
		return fmt.Errorf("failed to add token to Buildkite redactor: %w", err)
	}

	return nil
}
