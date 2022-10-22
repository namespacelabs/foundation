// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tui

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"syscall"

	"golang.org/x/term"
)

func AskSecret(ctx context.Context, title, desc, placeholder string) ([]byte, error) {
	if !term.IsTerminal(syscall.Stdin) {
		reader := bufio.NewReader(os.Stdin)
		// Read until (required) newline.
		s, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		return bytes.TrimSpace([]byte(s)), nil
	}

	secret, err := Ask(ctx, title, desc, placeholder)
	if err != nil {
		return nil, err
	}

	if secret == "" {
		return nil, context.Canceled
	}

	return bytes.TrimSpace([]byte(secret)), nil
}
