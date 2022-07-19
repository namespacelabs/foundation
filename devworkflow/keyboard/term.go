// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package keyboard

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/morikuni/aec"
	"github.com/muesli/cancelreader"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/termios"
	"namespacelabs.dev/foundation/internal/observers"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema"
)

type Handler interface {
	// Key MUST be a pure function. E.g. "l"
	Key() string
	// Label MUST be a pure function. E.g. "stream logs"
	Label(bool) string
	// Must only leave when chan Event is closed. OpSet events must be Acknowledged by writing to Control.
	Handle(context.Context, chan Event, chan<- Control)
}

type EventOp string
type ControlOp string

const (
	OpStackUpdate EventOp = "op.stackupdate"
	OpSet         EventOp = "op.set"

	ControlAck ControlOp = "control.ack"
)

type Event struct {
	HandlerID   int
	EventID     string
	Operation   EventOp
	StackUpdate struct {
		Env   *schema.Environment
		Stack *schema.Stack
		Focus []string
	}
	Enabled bool
}

type Control struct {
	Operation ControlOp
	AckEvent  struct {
		HandlerID int
		EventID   string
	}
}

// StartHandler processes user keystroke events and dev workflow updates.
// Here we also take care on calling `onDone` callback on user exiting.
func StartHandler(ctx context.Context, provider observers.SessionProvider, handlers []Handler, onDone func()) error {
	if !termios.IsTerm(os.Stdin.Fd()) {
		return nil
	}

	stdin, err := newStdinReader(ctx)
	if err != nil {
		return err
	}

	go handleEvents(ctx, stdin, provider, handlers, onDone)
	return nil
}

type handlerState struct {
	Handler       Handler
	HandlerID     int
	Ch            chan Event
	ExitCh        chan struct{}
	Enabled       bool
	HandlingEvent string
}

func handleEvents(ctx context.Context, stdin *rawStdinReader, provider observers.SessionProvider, handlers []Handler, onDone func()) {
	obs, err := provider.NewStackClient()
	if err != nil {
		fmt.Fprintln(console.Debug(ctx), "failed to create observer", err)
		return
	}

	defer obs.Close()
	defer stdin.restore()

	defer func() {
		if ctx.Err() == nil {
			onDone()
		}
	}()

	control := make(chan Control)
	state := make([]*handlerState, len(handlers))

	for k, handler := range handlers {
		st := &handlerState{
			Handler:   handler,
			HandlerID: k,
			Ch:        make(chan Event),
			ExitCh:    make(chan struct{}),
			Enabled:   false,
		}

		state[k] = st

		go func() {
			defer close(st.ExitCh)
			st.Handler.Handle(ctx, st.Ch, control)
		}()
	}

	defer func() {
		for _, state := range state {
			close(state.Ch)
		}

		for _, state := range state {
			<-state.ExitCh // Wait until the go routine exits.
		}

		// Only close control after all goroutines have exited.
		//
		// XXX there's a race condition here: if a go routine is waiting on
		// writing to control before returning, then we may never arrive here.
		close(control)
	}()

	eventID := uint64(0)
	for {
		var labels []string
		for _, state := range state {
			style := aec.DefaultF
			if state.HandlingEvent != "" {
				style = aec.LightBlackF
			}

			labels = append(labels, fmt.Sprintf(" (%s): %s", aec.Bold.Apply(state.Handler.Key()), style.Apply(state.Handler.Label(state.Enabled))))
		}

		keybindings := fmt.Sprintf(" %s%s (%s): quit", aec.LightBlackF.Apply("Key bindings"), strings.Join(labels, ""), aec.Bold.Apply("q"))
		console.SetStickyContent(ctx, "commands", keybindings)

		eventID++
		select {
		case update, ok := <-obs.StackEvents():
			if !ok {
				return
			}

			if len(state) > 0 {
				// Decouple changes made by devworkflow. Handlers should be able
				// to assume that the received event data is immutable.
				env := protos.Clone(update.Env)
				stack := protos.Clone(update.Stack)
				focus := slices.Clone(update.Focus)

				for _, handler := range state {
					ev := Event{
						HandlerID: handler.HandlerID,
						EventID:   fmt.Sprintf("%d", eventID),
						Operation: OpStackUpdate,
					}
					ev.StackUpdate.Env = env
					ev.StackUpdate.Stack = stack
					ev.StackUpdate.Focus = focus
					handler.Ch <- ev
				}
			}

		case err := <-stdin.errors:
			fmt.Fprintf(console.Errors(ctx), "Error while reading from Stdin: %v\n", err)
			return

		case c := <-stdin.input:
			if int(c) == 3 || string(c) == "q" { // ctrl+c
				return
			}

			key := string(c)

			for _, state := range state {
				if state.Handler.Key() != key {
					continue
				}

				// Don't allow multiple state changes until the handler acknowledges the command.
				if state.HandlingEvent == "" {
					state.HandlingEvent = fmt.Sprintf("%d", eventID)
					state.Enabled = !state.Enabled

					state.Ch <- Event{
						HandlerID: state.HandlerID,
						EventID:   state.HandlingEvent,
						Operation: OpSet,
						Enabled:   state.Enabled,
					}
				}

				break
			}

		case c := <-control:
			if c.Operation == ControlAck {
				if c.AckEvent.HandlerID < 0 || c.AckEvent.HandlerID >= len(state) {
					panic("handler id is invalid")
				}

				state := state[c.AckEvent.HandlerID]
				if state.HandlingEvent == c.AckEvent.EventID {
					state.HandlingEvent = ""
				}
			}

		case <-ctx.Done():
			return
		}
	}
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
