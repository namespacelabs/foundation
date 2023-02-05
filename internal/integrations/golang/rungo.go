// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package golang

import (
	"context"
	"strings"

	"namespacelabs.dev/foundation/internal/git"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/sdk/golang"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

func RunGo(ctx context.Context, loc pkggraph.Location, sdk golang.LocalSDK, args ...string) error {
	return tasks.Action("go.run").Arg("dir", loc.Rel()).HumanReadablef("go "+strings.Join(args, " ")).Run(ctx, func(ctx context.Context) error {
		var cmd localexec.Command
		cmd.Command = golang.GoBin(sdk)
		cmd.Args = args
		cmd.AdditionalEnv = MakeGoEnv(sdk)
		cmd.Dir = loc.Abs()
		cmd.Label = "go " + strings.Join(cmd.Args, " ")
		return cmd.Run(ctx)
	})
}

func MakeGoEnv(sdk golang.LocalSDK) []string {
	return append([]string{golang.GoRootEnv(sdk), goPrivate()}, git.NoPromptEnv().Serialize()...)
}
