// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package git

import (
	"fmt"
	"os"
)

var AssumeSSHAuth = false

func NoPromptEnv() []string {
	// Disable password promts as we don't handle them properly, yet.
	env := []string{"GIT_TERMINAL_PROMPT=0"}

	// Also disable prompting for passwords by the 'ssh' subprocess spawned by Git.
	//
	// See https://github.com/golang/go/blob/fad67f8a5342f4bc309f26f0ae021ce9d21724e6/src/cmd/go/internal/get/get.go#L129
	if os.Getenv("GIT_SSH") == "" && os.Getenv("GIT_SSH_COMMAND") == "" {
		env = append(env, "GIT_SSH_COMMAND=ssh -o ControlMaster=no -o BatchMode=yes")
	}

	// And one more source of Git prompts: the Git Credential Manager Core for Windows.
	//
	// See https://github.com/microsoft/Git-Credential-Manager-Core/blob/master/docs/environment.md#gcm_interactive.
	if os.Getenv("GCM_INTERACTIVE") == "" {
		env = append(env, "GCM_INTERACTIVE=never")
	}

	var overrides [][2]string
	if AssumeSSHAuth {
		// XXX make this an input parameter.
		overrides = append(overrides,
			[2]string{`url.git@github.com:namespacelabs/.insteadOf`, "https://github.com/namespacelabs/"},
		)
	}

	env = append(env, fmt.Sprintf("GIT_CONFIG_COUNT=%d", len(overrides)))
	for k, override := range overrides {
		env = append(env, fmt.Sprintf("GIT_CONFIG_KEY_%d=%s", k, override[0]))
		env = append(env, fmt.Sprintf("GIT_CONFIG_VALUE_%d=%s", k, override[1]))
	}

	return env
}
