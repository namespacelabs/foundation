// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package termios

import (
	"golang.org/x/sys/unix"
)

const (
	ioctl_GETATTR = unix.TCGETS
	ioctl_SETATTR = unix.TCSETS
)