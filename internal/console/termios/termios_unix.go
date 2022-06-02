// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package termios

import (
	"golang.org/x/sys/unix"
)

func IsTerm(fd uintptr) bool {
	if _, err := unix.IoctlGetTermios(int(fd), ioctlGETATTR); err != nil {
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

func MakeRaw(fd uintptr) (func() error, error) {
	termios, err := unix.IoctlGetTermios(int(fd), ioctlGETATTR)
	if err != nil {
		return nil, err
	}

	oldState := *termios

	// This attempts to replicate the behaviour documented for cfmakeraw in
	// the termios(3) manpage.
	termios.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	termios.Oflag &^= unix.OPOST
	termios.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	termios.Cflag &^= unix.CSIZE | unix.PARENB
	termios.Cflag |= unix.CS8
	termios.Cc[unix.VMIN] = 1
	termios.Cc[unix.VTIME] = 0

	termios.Oflag |= unix.OPOST // Don't change how \r is treated.

	if err := unix.IoctlSetTermios(int(fd), ioctlSETATTR, termios); err != nil {
		return nil, err
	}

	return func() error {
		return unix.IoctlSetTermios(int(fd), ioctlSETATTR, &oldState)
	}, nil
}
