// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package ulimit

import (
	"context"
	"fmt"
	"syscall"

	"namespacelabs.dev/foundation/internal/console"
)

func SetFileLimit(ctx context.Context, n uint64) {
	if err := setFileLimit(n); err != nil {
		fmt.Fprintf(console.Debug(ctx), "Failed to set ulimit on number of open files to %d: %v\n", n, err)
	}
}

func setFileLimit(n uint64) error {
	var rlimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit); err != nil {
		return err
	}

	// Already running with a limit that meets the requested number.
	if rlimit.Cur >= n {
		return nil
	}

	newLimit := syscall.Rlimit{Cur: n, Max: n}
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &newLimit); err != nil {
		return err
	}

	return nil
}
