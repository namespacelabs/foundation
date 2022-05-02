package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	cons "github.com/containerd/console"
	"github.com/muesli/cancelreader"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

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
		current.Reset()
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

func termCommands(ctx context.Context, rt runtime.Runtime, serverProtos []*schema.Server, onExit func()) {
	r, err := newReader(ctx)
	if err != nil {
		fmt.Fprintln(console.Errors(ctx), err)
		onExit()
		return
	}
	defer r.restore()

	observeStarted := false
	for {
		select {
		case err := <-r.errors:
			fmt.Fprintf(console.Errors(ctx), "Error while reading from Stdin: %v:", err)
			onExit()
			return
		case c := <-r.input:
			if int(c) == 3 {
				onExit()
				return
			}
			if string(c) == "q" {
				onExit()
				return
			}
			if string(c) == "l" && !observeStarted {
				observeStarted = true
				for _, server := range serverProtos {
					go observeLogs(ctx, rt, server)
				}
				// TODO handle multiple keystrokes.
			}
		case <-ctx.Done():
			r.cancel()
			return
		}

	}
}

// observeLogs observes a given server in a given runtime and writes the logs to `console.Output`.
func observeLogs(ctx context.Context, rt runtime.Runtime, serverProto *schema.Server) error {
	streams := map[string]*logStream{}
	var mu sync.Mutex
	return rt.Observe(ctx, serverProto, runtime.ObserveOpts{}, func(ev runtime.ObserveEvent) error {
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
