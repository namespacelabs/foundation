// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package localexec

import (
	"context"
	"os"
	"os/exec"

	"namespacelabs.dev/foundation/internal/console"
)

func RunInteractive(ctx context.Context, cmd *exec.Cmd) error {
	done := console.EnterInputMode(ctx)
	defer done()

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
