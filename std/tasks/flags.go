// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tasks

import "github.com/spf13/pflag"

func SetupFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&LogActions, "log_actions", LogActions, "If set to true, each completed action is also output as a log message.")

	flags.Int32Var(&concurrentBuilds, "concurrent_build_limit", concurrentBuilds, "Limit how many builds may run in parallel.")
	_ = flags.MarkHidden("concurrent_build_limit")
}
