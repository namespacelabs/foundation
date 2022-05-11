// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package logs

import (
	"context"
	"fmt"
	"os"

	cons "github.com/containerd/console"
	"github.com/morikuni/aec"
	"github.com/muesli/cancelreader"
	"namespacelabs.dev/foundation/devworkflow"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

type Term interface {
	// Renders available commands.
	PrintCommands(ctx context.Context)
	// Hanles user input events and changing environment.
	HandleEvents(ctx context.Context,
		root *workspace.Root, serverProtos []*schema.Server, onDone func(), ch chan *devworkflow.Update)
}

func NewTerm() Term {
	return &term{}
}

type term struct {
	cancelFuncs []context.CancelFunc
}

// HandleEvents processes user keystroke events and dev workflow updates.
// Here we also take care on calling `onDone` callback on user exiting.
func (t *term) HandleEvents(ctx context.Context, root *workspace.Root, serverProtos []*schema.Server, onDone func(), ch chan *devworkflow.Update) {
	r, err := newStdinReader(ctx)
	if err != nil {
		fmt.Fprintln(console.Errors(ctx), err)
		return
	}
	defer r.restore()
	defer func() {
		if ctx.Err() == nil {
			onDone()
		}
	}()

	envRef := ""
	showingLogs := false
	for {
		select {
		case update := <-ch:
			if update.StackUpdate != nil && update.StackUpdate.Env != nil {
				if showingLogs && envRef != update.StackUpdate.Env.Name {
					t.stopLogging()
					t.newLogTailMultiple(ctx, root, update.StackUpdate.Env.Name, serverProtos)
				}
				envRef = update.StackUpdate.Env.Name
			}
		case err := <-r.errors:
			fmt.Fprintf(console.Errors(ctx), "Error while reading from Stdin: %v\n", err)
			return
		case c := <-r.input:
			if int(c) == 3 || string(c) == "q" { // ctrl+c
				t.stopLogging()
				return
			}
			if string(c) == "l" && envRef != "" && !showingLogs {
				showingLogs = true
				t.newLogTailMultiple(ctx, root, envRef, serverProtos)
				// TODO handle multiple keystrokes.
			}
		case <-ctx.Done():
			r.cancel()
			return
		}
	}
}

func (t term) PrintCommands(ctx context.Context) {
	cmds := fmt.Sprintf(" (%s): logs (%s): quit", aec.Bold.Apply("l"), aec.Bold.Apply("q"))
	updateCmd(ctx, cmds)
}

func (t *term) newLogTailMultiple(ctx context.Context, root *workspace.Root, envRef string, serverProtos []*schema.Server) {
	for _, server := range serverProtos {
		server := server // Capture server
		ctxWithCancel, cancelF := context.WithCancel(ctx)
		t.cancelFuncs = append(t.cancelFuncs, cancelF)
		go func() {
			err := NewLogTail(ctxWithCancel, root, envRef, server)
			if err != nil {
				fmt.Fprintf(console.Errors(ctx), "Error starting logs: %v\n", err)
			}
		}()
	}
}

func (t *term) stopLogging() {
	for _, cancelF := range t.cancelFuncs {
		cancelF()
	}
	t.cancelFuncs = t.cancelFuncs[:]
}

func updateCmd(ctx context.Context, cmd string) {
	console.SetStickyContent(ctx, "cmds", []byte(cmd))
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
	current := cons.Current()
	if err := current.SetRaw(); err != nil {
		return nil, err
	}
	r.restore = func() {
		cr.Cancel()
		if err := current.Reset(); err != nil {
			fmt.Fprintf(console.Errors(ctx), "Error : %v:", err)
		}
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
