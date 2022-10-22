// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

//go:build !windows && !linux
// +build !windows,!linux

package termios

import (
	"golang.org/x/sys/unix"
)

const (
	ioctlGETATTR = unix.TIOCGETA
	ioctlSETATTR = unix.TIOCSETA
)
