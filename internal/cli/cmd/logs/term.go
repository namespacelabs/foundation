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
	Commands(ctx context.Context)
	// Hanles user input events and changing environment.
	HandleEvents(ctx context.Context,
		root *workspace.Root, serverProtos []*schema.Server, onDone func(), ch chan *devworkflow.Update)
}

type term struct{}

func NewTerm() Term {
	return &term{}
}

// TermCommands processes user commands and dev workflow updates.
func (t *term) HandleEvents(ctx context.Context, root *workspace.Root, serverProtos []*schema.Server, onDone func(), ch chan *devworkflow.Update) {
	r, err := newReader(ctx)
	if err != nil {
		fmt.Fprintln(console.Errors(ctx), err)
		return
	}
	defer r.restore()
	defer onDone()

	envRef := ""
	showingLogs := false
	logsObserver := NewLogsObserver()
	for {
		select {
		case update := <-ch:
			if update.StackUpdate != nil && update.StackUpdate.Env != nil {
				envRef = update.StackUpdate.Env.Name
				if !showingLogs {
					continue
				}
				logsObserver.Stop()
				logsObserver.Start(ctx, root, envRef, serverProtos)
			}
		case err := <-r.errors:
			fmt.Fprintf(console.Errors(ctx), "Error while reading from Stdin: %v:", err)
			return
		case c := <-r.input:
			if int(c) == 3 || string(c) == "q" { // ctrl+c
				updateCmd(ctx, " Quitting...")
				return
			}
			if string(c) == "l" && envRef != "" && !showingLogs {
				showingLogs = true
				logsObserver.Start(ctx, root, envRef, serverProtos)
				// TODO handle multiple keystrokes.
			}
		case <-ctx.Done():
			r.cancel()
			return
		}
	}
}

func (t term) Commands(ctx context.Context) {
	cmds := fmt.Sprintf(" (%s): logs (%s): quit", aec.Bold.Apply("l"), aec.Bold.Apply("q"))
	updateCmd(ctx, cmds)
}

func updateCmd(ctx context.Context, cmd string) {
	console.SetStickyContent(ctx, "cmds", []byte(cmd))
}

type rawReader struct {
	input   chan byte
	errors  chan error
	cancel  func() bool
	restore func()
}

func newReader(ctx context.Context) (*rawReader, error) {
	cr, err := cancelreader.NewReader(os.Stdin)
	if err != nil {
		return nil, err
	}

	r := &rawReader{
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
