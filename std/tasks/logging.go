// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/environment"
)

var LogActions = environment.IsRunningInCI()

func SetupFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&LogActions, "log_actions", LogActions, "If set to true, each completed action is also output as a log message.")
}
