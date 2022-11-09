// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package keyboard

import (
	"context"
	"io"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/creack/pty"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/observers"
)

func TestHandleExitsWhenHandlerErrs(t *testing.T) {
	// Catch the condition where the handler passed to Handle() exits,
	// but Handle continues to pump events that go nowhere.
	// In this case the error message is swallowed and not displayed
	// and also all keyboard input (including ^C) is blocked.

	ctx := context.Background()

	// Set up stdin TTY so that tea.Program can (try to) read from it.
	pty, tty, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	oldStdin := os.Stdin
	os.Stdin = tty
	go func() {
		if _, err := io.Copy(pty, os.Stdout); err != nil {
			log.Printf("copy failed with %v", err)
		}
	}()
	defer func() {
		os.Stdin = oldStdin
		tty.Close()
		pty.Close()
	}()

	// This method blocks if there's a bug. Use `go test -timeout 5s` to run the
	// test with a lower timeout, for debugging.
	if err := Handle(ctx, HandleOpts{
		Provider:    &fakeProvider{},
		Keybindings: []Handler{},
		Handler: func(context.Context) error {
			return fnerrors.New("expected-in-test")
		},
	}); err == nil {
		t.Fatal("expected an error")
	} else if !strings.Contains(err.Error(), "expected-in-test") {
		t.Errorf("Unexpected error from Handle: %v", err)
	}

}

type fakeProvider struct{}

func (*fakeProvider) NewStackClient() (observers.StackSession, error) {
	return &fakeSession{make(chan *observers.StackUpdateEvent)}, nil
}

type fakeSession struct {
	ch chan *observers.StackUpdateEvent
}

func (f *fakeSession) Close()                                        { close(f.ch) }
func (f *fakeSession) StackEvents() chan *observers.StackUpdateEvent { return f.ch }
