// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package golang

import (
	"context"
	"strings"

	"namespacelabs.dev/foundation/internal/git"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/sdk/golang"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func RunGo(ctx context.Context, loc pkggraph.Location, sdk golang.LocalSDK, args ...string) error {
	return tasks.Action("go.run").Arg("dir", loc.Rel()).HumanReadablef("go "+strings.Join(args, " ")).Run(ctx, func(ctx context.Context) error {
		var cmd localexec.Command
		cmd.Command = sdk.GoBin()
		cmd.Args = args
		cmd.AdditionalEnv = makeGoEnv(sdk)
		cmd.Dir = loc.Abs()
		cmd.Label = "go " + strings.Join(cmd.Args, " ")
		return cmd.Run(ctx)
	})
}

func makeGoEnv(sdk golang.LocalSDK) []string {
	return append([]string{sdk.GoRootEnv(), goPrivate()}, git.NoPromptEnv().Serialize()...)
}
