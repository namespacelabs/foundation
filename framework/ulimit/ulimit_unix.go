// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

//go:build unix

package ulimit

import (
	"syscall"
)

func SetFileLimit(n uint64) error {
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

func SetCoreFileLimit(maxCoreFileSize uint64) error {
	var rlimit syscall.Rlimit

	if err := syscall.Getrlimit(syscall.RLIMIT_CORE, &rlimit); err != nil {
		return err
	}

	if rlimit.Max < maxCoreFileSize {
		rlimit.Max = maxCoreFileSize
	}

	if rlimit.Cur < maxCoreFileSize {
		rlimit.Cur = maxCoreFileSize
	}

	if err := syscall.Setrlimit(syscall.RLIMIT_CORE, &rlimit); err != nil {
		return err
	}

	return nil
}
