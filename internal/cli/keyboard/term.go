// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package keyboard

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/morikuni/aec"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/termios"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/observers"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
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
		Env         *schema.Environment
		Stack       *schema.Stack
		Focus       []string
		NetworkPlan *storage.NetworkPlan
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

type HandleOpts struct {
	Provider    observers.SessionProvider
	Keybindings []Handler
	Handler     func(context.Context) error
}

// Handle processes user keystroke events and dev workflow updates. Here we also
// take care on calling `onDone` callback on user exiting.
func Handle(ctx context.Context, opts HandleOpts) error {
	if !termios.IsTerm(os.Stdin.Fd()) {
		return opts.Handler(ctx)
	}

	obs, err := opts.Provider.NewStackClient()
	if err != nil {
		return err
	}

	defer obs.Close()

	keych := make(chan tea.KeyMsg)
	p := tea.NewProgram(&program{ch: keych, w: console.Stderr(ctx)}, tea.WithoutRenderer())

	ctx, cancelContext := context.WithCancel(ctx)
	defer cancelContext()

	go handleEvents(ctx, obs, opts.Keybindings, keych)

	eg := executor.New(ctx, "keyboard-handler")
	eg.Go(opts.Handler)
	eg.Go(func(ctx context.Context) error {
		m, err := p.StartReturningModel()
		if err != nil {
			return err
		}

		if m.(*program).quit {
			return context.Canceled
		}

		return nil
	})

	return eg.Wait()
}

type program struct {
	ch   chan tea.KeyMsg
	quit bool
	w    io.Writer
}

func (m *program) Init() tea.Cmd { return nil }
func (m *program) View() string  { return "" }

func (m *program) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.quit = true
			return m, tea.Quit

		default:
			if msg.String() == "q" {
				m.quit = true
				return m, tea.Quit
			}

			m.ch <- msg
		}

	case tea.WindowSizeMsg:
	}

	return m, nil
}

type handlerState struct {
	Handler       Handler
	HandlerID     int
	Ch            chan Event
	ExitCh        chan struct{}
	Enabled       bool
	HandlingEvent string
}

func handleEvents(ctx context.Context, obs observers.StackSession, handlers []Handler, keych chan tea.KeyMsg) {
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
				// Decouple changes made by devsession. Handlers should be able
				// to assume that the received event data is immutable.
				env := protos.Clone(update.Env)
				stack := protos.Clone(update.Stack)
				focus := slices.Clone(update.Focus)
				networkPlan := protos.Clone(update.NetworkPlan)

				for _, handler := range state {
					ev := Event{
						HandlerID: handler.HandlerID,
						EventID:   fmt.Sprintf("%d", eventID),
						Operation: OpStackUpdate,
					}
					ev.StackUpdate.Env = env
					ev.StackUpdate.Stack = stack
					ev.StackUpdate.Focus = focus
					ev.StackUpdate.NetworkPlan = networkPlan
					handler.Ch <- ev
				}
			}

		case key, ok := <-keych:
			if !ok {
				return
			}

			for _, state := range state {
				if state.Handler.Key() != key.String() {
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
