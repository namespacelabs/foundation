// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package logs

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	cons "github.com/containerd/console"
	"github.com/morikuni/aec"
	"github.com/muesli/cancelreader"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

// TermCommands processes user commands issued from the terminal.
// On successful exit, the terminal is reset to its original state and `onDone()` callback is called.
func TermCommands(ctx context.Context, serverProtos []*schema.Server, start chan bool, onDone func()) {
	r, err := newReader(ctx)
	if err != nil {
		fmt.Fprintln(console.Errors(ctx), err)
		return
	}
	defer r.restore()
	prefix := aec.Bold.Apply(" Commands:")
	commands := fmt.Sprintf("%s (%s)ogs (%s)uit", prefix, aec.Bold.Apply("l"), aec.Bold.Apply("q"))
	console.SetStickyContent(ctx, "cmds", []byte(commands))
	showingLogs := false
	for {
		select {
		case err := <-r.errors:
			fmt.Fprintf(console.Errors(ctx), "Error while reading from Stdin: %v:", err)
			return
		case c := <-r.input:
			if int(c) == 3 { // ctrl+c
				console.SetStickyContent(ctx, "cmds", []byte(fmt.Sprintf("%s Pressed ctrl+c. Quitting...", prefix)))
				onDone()
				return
			}
			if string(c) == "q" { // ctrl+c
				console.SetStickyContent(ctx, "cmds", []byte(fmt.Sprintf("%s Quitting...", prefix)))
				onDone()
				return
			}
			if string(c) == "l" && !showingLogs {
				showingLogs = true
				console.SetStickyContent(ctx, "cmds", []byte(fmt.Sprintf("%s Showing logs...", prefix)))
				start <- true
				// TODO handle multiple keystrokes.
			}
		case <-ctx.Done():
			r.cancel()
			return
		}

	}
}

// ObserveLogs observes a given server in a given runtime and writes the logs to `console.Output`.
func ObserveLogs(ctx context.Context, serverProtos []*schema.Server, start chan bool) func(rt runtime.Runtime) {
	return func(rt runtime.Runtime) {
		select {
		case <-ctx.Done():
			return
		case <-start:
			for _, server := range serverProtos {
				server := server
				go func() {
					if err := observeServer(ctx, rt, server, start); err != nil {
						fmt.Fprintf(console.Errors(ctx), "Error while observing logs: %v", err)
					}
				}()
			}
		}
	}
}

func ObserveLogsSingleServr(ctx context.Context, rt runtime.Runtime, server *schema.Server) error {
	start := make(chan bool)
	start <- true
	return observeServer(ctx, rt, server, start)
}

func observeServer(ctx context.Context, rt runtime.Runtime, server *schema.Server, start chan bool) error {
	var mu sync.Mutex
	streams := map[string]*logStream{}
	return rt.Observe(ctx, server, runtime.ObserveOpts{}, func(ev runtime.ObserveEvent) error {
		mu.Lock()
		existing := streams[ev.ContainerReference.UniqueID()]
		if ev.Removed {
			delete(streams, ev.ContainerReference.UniqueID())
		}
		mu.Unlock()

		if ev.Added {
			if existing != nil {
				return nil
			}
		} else if ev.Removed {
			if existing != nil {
				existing.cancel()
			}
			return nil
		}

		newS := &logStream{}
		mu.Lock()
		streams[ev.ContainerReference.UniqueID()] = newS
		mu.Unlock()

		compute.On(ctx).Detach(tasks.Action("stream-log").Indefinite(), func(ctx context.Context) error {
			w := console.Output(ctx, ev.HumanReadableID)
			ctx, cancel := context.WithCancel(ctx)

			if !newS.set(cancel, w) {
				// Raced with pod disappearing.
				return nil
			}

			fmt.Fprintln(w, "<Starting to stream>")

			return rt.FetchLogsTo(ctx, w, ev.ContainerReference, runtime.FetchLogsOpts{
				TailLines: 20,
				Follow:    true,
			})
		})

		return nil
	})
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

type logStream struct {
	mu         sync.Mutex
	cancelFunc func()
	cancelled  bool
	w          io.Writer
}

func (ls *logStream) cancel() {
	ls.mu.Lock()
	cancel := ls.cancelFunc
	ls.cancelFunc = nil
	wasCancelled := ls.cancelled
	ls.cancelled = true
	w := ls.w
	ls.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if !wasCancelled {
		fmt.Fprintln(w, "<Closed>")
	}
}

func (ls *logStream) set(cancel func(), w io.Writer) bool {
	ls.mu.Lock()
	cancelled := ls.cancelled
	ls.cancelFunc = cancel
	ls.w = w
	ls.mu.Unlock()
	return !cancelled
}
