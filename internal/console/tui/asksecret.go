package tui

import (
	"bufio"
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
		return []byte(s), nil
	}

	secret, err := Ask(ctx, title, desc, placeholder)
	if err != nil {
		return nil, err
	}

	if secret == "" {
		return nil, context.Canceled
	}

	return []byte(secret), nil
}
