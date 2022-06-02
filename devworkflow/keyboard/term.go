// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package keyboard

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/morikuni/aec"
	"github.com/muesli/cancelreader"
	"namespacelabs.dev/foundation/devworkflow"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/termios"
	"namespacelabs.dev/foundation/internal/logs/logtail"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

// StartHandler processes user keystroke events and dev workflow updates.
// Here we also take care on calling `onDone` callback on user exiting.
func StartHandler(ctx context.Context, stackState *devworkflow.Session, root *workspace.Root, serverProtos []*schema.Server, onDone func()) error {
	if !termios.IsTerm(os.Stdin.Fd()) {
		return nil
	}

	stdin, err := newStdinReader(ctx)
	if err != nil {
		return err
	}

	t := &termState{}
	go t.handleEvents(ctx, stdin, stackState, root, serverProtos, onDone)
	return nil
}

type termState struct {
	cancelFuncs []context.CancelFunc
	showingLogs bool
}

func (t *termState) handleEvents(ctx context.Context, stdin *rawStdinReader, stackState *devworkflow.Session, root *workspace.Root, serverProtos []*schema.Server, onDone func()) {
	obs, err := stackState.NewClient(false)
	if err != nil {
		fmt.Fprintln(console.Debug(ctx), "failed to create observer", err)
		return
	}

	defer obs.Close()

	t.updateSticky(ctx)

	defer stdin.restore()

	defer func() {
		if ctx.Err() == nil {
			onDone()
		}
	}()

	defer t.stopLogging()

	envRef := ""
	for {
		select {
		case update, ok := <-obs.Events():
			if !ok {
				return
			}

			if update.StackUpdate != nil && update.StackUpdate.Env != nil {
				if t.showingLogs && envRef != update.StackUpdate.Env.Name {
					t.stopLogging()
					t.newLogTailMultiple(ctx, root, update.StackUpdate.Env.Name, serverProtos)
				}
				envRef = update.StackUpdate.Env.Name
			}

		case err := <-stdin.errors:
			fmt.Fprintf(console.Errors(ctx), "Error while reading from Stdin: %v\n", err)
			return

		case c := <-stdin.input:
			if int(c) == 3 || string(c) == "q" { // ctrl+c
				return
			}

			if string(c) == "l" && envRef != "" {
				if t.showingLogs {
					t.stopLogging()
				} else {
					t.newLogTailMultiple(ctx, root, envRef, serverProtos)
				}

				t.showingLogs = !t.showingLogs
				t.updateSticky(ctx)
			}

		case <-ctx.Done():
			return
		}
	}
}

func (t *termState) updateSticky(ctx context.Context) {
	logCmd := "stream logs"
	if t.showingLogs {
		logCmd = "pause logs " // Additional space at the end for a better allignment.
	}

	keybindings := fmt.Sprintf(" %s (%s): %s (%s): quit", aec.LightBlackF.Apply("Key bindings"), aec.Bold.Apply("l"), logCmd, aec.Bold.Apply("q"))
	console.SetStickyContent(ctx, "commands", []byte(keybindings))
}

func (t *termState) newLogTailMultiple(ctx context.Context, root *workspace.Root, envRef string, serverProtos []*schema.Server) {
	for _, server := range serverProtos {
		server := server // Capture server
		ctxWithCancel, cancelF := context.WithCancel(ctx)
		t.cancelFuncs = append(t.cancelFuncs, cancelF)
		go func() {
			env, err := provision.RequireEnv(root, envRef)
			if err == nil {
				err = logtail.Listen(ctxWithCancel, env, server)
			}

			if err != nil && !errors.Is(err, context.Canceled) {
				fmt.Fprintf(console.Errors(ctx), "Error starting logs: %v\n", err)
			}
		}()
	}
}

func (t *termState) stopLogging() {
	for _, cancelF := range t.cancelFuncs {
		cancelF()
	}
	t.cancelFuncs = nil
}

type rawStdinReader struct {
	input   chan byte
	errors  chan error
	cancel  func() bool
	restore func()
}

func newStdinReader(ctx context.Context) (*rawStdinReader, error) {
	cr, err := cancelreader.NewReader(os.Stdin)
	if err != nil {
		return nil, err
	}

	r := &rawStdinReader{
		input:  make(chan byte),
		cancel: cr.Cancel,
	}

	restore, err := termios.MakeRaw(os.Stdin.Fd())
	if err != nil {
		return nil, err
	}

	r.restore = func() {
		cr.Cancel()
		_ = restore()
	}

	go func() {
		var buf [256]byte
		for {
			_, err := cr.Read(buf[:])
			if err != nil {
				r.errors <- err
				return
			}
			r.input <- buf[0]
		}
	}()

	return r, nil
}
