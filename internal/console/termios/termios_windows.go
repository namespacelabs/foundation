// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows
// +build windows

package termios

import "golang.org/x/sys/windows"

func IsTerm(fd uintptr) bool {
	var st uint32
	err := windows.GetConsoleMode(windows.Handle(fd), &st)
	return err == nil
}

func TermSize(fd uintptr) (WinSize, error) {
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(fd), &info); err != nil {
		return WinSize{}, err
	}
	var ts WinSize
	ts.Height = uint16(info.Window.Bottom - info.Window.Top + 1)
	ts.Width = uint16(info.Window.Right - info.Window.Left + 1)

	return ts, nil
}

func MakeRaw(fd uintptr) (func() error, error) {
	var oldState uint32
	if err := windows.GetConsoleMode(windows.Handle(fd), &oldState); err != nil {
		return nil, err
	}

	raw := oldState &^ (windows.ENABLE_ECHO_INPUT | windows.ENABLE_PROCESSED_INPUT | windows.ENABLE_LINE_INPUT | windows.ENABLE_PROCESSED_OUTPUT)
	if err := windows.SetConsoleMode(windows.Handle(fd), raw); err != nil {
		return nil, err
	}

	return func() error {
		return windows.SetConsoleMode(windows.Handle(oldState), raw)
	}, nil
}
