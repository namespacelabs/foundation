// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package clerk

import (
	"github.com/spf13/pflag"
)

var devClerk = false

func SetupFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&devClerk, "dev_clerk", devClerk, "Use DEV Clerk instance.")
	_ = flags.MarkHidden("dev_clerk")
}

type State struct {
	Email          string `json:"email,omitempty"`
	Name           string `json:"name,omitempty"`
	ClerkClient    string `json:"clerk_client,omitempty"`
	GithubUsername string `json:"github_username,omitempty"`
	DevSession     string `json:"dev_session,omitempty"` // Only for DEV instance
}
