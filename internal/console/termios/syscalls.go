// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

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
