// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package termios

import (
	"os"

	"golang.org/x/term"
)

func TermSize(fd uintptr) (WinSize, error) {
	w, h, err := term.GetSize(int(fd))
	return WinSize{Width: uint16(w), Height: uint16(h)}, err
}

func IsTerm(fd uintptr) bool {
	return term.IsTerminal(int(fd))
}

func NotifyWindowSize(ch chan<- os.Signal) {
	// Not implemented.
}

func MakeRaw(fd uintptr) (func() error, error) {
	st, err := term.MakeRaw(int(fd))
	if err != nil {
		return nil, err
	}

	return func() error {
		return term.Restore(int(fd), st)
	}, nil
}
