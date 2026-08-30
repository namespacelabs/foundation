// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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
