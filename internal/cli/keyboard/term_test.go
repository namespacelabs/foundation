// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package keyboard

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

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
	go io.Copy(pty, os.Stdout)
	defer func() {
		os.Stdin = oldStdin
		tty.Close()
		pty.Close()
	}()

	ret := make(chan error)
	go func() {
		// This goroutine will get stuck forever if there is a logic error.
		ret <- Handle(ctx, HandleOpts{
			Provider:    &fakeProvider{},
			Keybindings: []Handler{},
			Handler: func(context.Context) error {
				return fnerrors.New("expected-in-test")
			},
		})
	}()

	select {
	case err = <-ret:
		if !strings.Contains(err.Error(), "expected-in-test") {
			t.Errorf("Unexpected error from Handle: %v", err)
		}
	case <-time.After(1 * time.Second):
		// Yes, this is indeed a race and 1s is an arbitrary time limit
		// on how long it takes to return from the errgroup.
		t.Fatal("Handle didn't finish within 1s")
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
