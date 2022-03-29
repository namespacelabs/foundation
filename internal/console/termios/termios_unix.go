// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package termios

import (
	"golang.org/x/sys/unix"
)

func IsTerm(fd uintptr) bool {
	if _, err := unix.IoctlGetTermios(int(fd), ioctl_GETATTR); err != nil {
		return false
	}
	return true
}

func TermSize(fd uintptr) (WinSize, error) {
	uws, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	if err != nil {
		return WinSize{}, err
	}

	var ts WinSize
	ts.Height = uws.Row
	ts.Width = uws.Col
	return ts, nil
}