// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package termios

import (
	"os"

	"golang.org/x/term"
)

func MakeRaw(f *os.File) (func() error, error) {
	oldState, err := term.MakeRaw(int(f.Fd()))
	if err != nil {
		panic(err)
	}

	return func() error { return term.Restore(int(f.Fd()), oldState) }, nil
}
