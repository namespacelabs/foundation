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
	"sync"

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
	// Must only leave when chan Event is closed. OpSet events must be Acknowledged by writing to Control.
	Handle(context.Context, chan Event, chan<- Control)

	// The first state is the initial state as well.
	States() []HandlerState
}

type HandlerState struct {
	State string
	Label string
}

type EventOp string
type ControlOp string

const (
	OpStackUpdate EventOp = "op.stackupdate"
	OpSet         EventOp = "op.set"
)

type Event struct {
	EventID     string
	Operation   EventOp
	StackUpdate struct {
		Env         *schema.Environment
		Stack       *schema.Stack
		Deployed    bool
		Focus       []string
		NetworkPlan *storage.NetworkPlan
	}
	CurrentState string
}

type Control struct {
	HandlerID int
	AckEvent  struct {
		EventID string
	}
	SetEnabled *bool
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
	p := tea.NewProgram(&program{ch: keych, w: console.Stderr(ctx)}, tea.WithoutRenderer(), tea.WithContext(ctx))

	ctx, cancelContext := context.WithCancel(ctx)
	defer cancelContext()

	go handleEvents(ctx, obs, opts.Keybindings, keych)

	eg := executor.New(ctx, "keyboard-handler")
	eg.Go(opts.Handler)
	eg.Go(func(ctx context.Context) error {
		defer close(keych)
		m, err := p.Run()
		if err == tea.ErrProgramKilled {
			return context.Canceled
		} else if err != nil {
			return err
		} else if m.(*program).quit {
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

type internalState struct {
	Handler       Handler
	HandlerID     int
	Ch            chan Event
	Enabled       bool
	HandlingEvent string
	States        []HandlerState
	Current       int
}

func handleEvents(ctx context.Context, obs observers.StackSession, handlers []Handler, keych chan tea.KeyMsg) {
	control := make(chan Control)
	state := make([]*internalState, len(handlers))

	var goroutines sync.WaitGroup

	for k, handler := range handlers {
		k := k // Close k.
		st := &internalState{
			Handler:   handler,
			HandlerID: k,
			Ch:        make(chan Event, 1),
			States:    handler.States(),
			Current:   0,
			Enabled:   false,
		}

		state[k] = st

		goroutines.Add(1)
		go func() {
			handlerControl := make(chan Control, 1)

			go func() {
				for op := range handlerControl {
					op.HandlerID = k
					control <- op
				}

				goroutines.Done()
			}()

			st.Handler.Handle(ctx, st.Ch, handlerControl)
		}()
	}

	defer func() {
		for _, state := range state {
			close(state.Ch)
		}

		// Wait until the go routine exits.
		goroutines.Wait()

		// Only close control after all goroutines have exited.
		close(control)
	}()

	eventID := uint64(0)
	for {
		var labels []string
		for _, state := range state {
			if !state.Enabled || state.HandlingEvent != "" {
				labels = append(labels, aec.LightBlackF.Apply(fmt.Sprintf(" (%s): %s", state.Handler.Key(), state.States[state.Current].Label)))
			} else {
				labels = append(labels, fmt.Sprintf(" (%s): %s", aec.Bold.Apply(state.Handler.Key()), state.States[state.Current].Label))
			}
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
						EventID:   fmt.Sprintf("%d", eventID),
						Operation: OpStackUpdate,
					}
					ev.StackUpdate.Env = env
					ev.StackUpdate.Stack = stack
					ev.StackUpdate.Focus = focus
					ev.StackUpdate.NetworkPlan = networkPlan
					ev.StackUpdate.Deployed = update.Deployed
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
				if state.HandlingEvent == "" && state.Enabled {
					state.HandlingEvent = fmt.Sprintf("%d", eventID)

					state.Current++
					if state.Current >= len(state.States) {
						state.Current = 0
					}

					state.Ch <- Event{
						EventID:      state.HandlingEvent,
						Operation:    OpSet,
						CurrentState: state.States[state.Current].State,
					}
				}

				break
			}

		case c := <-control:
			if c.HandlerID < 0 || c.HandlerID >= len(state) {
				panic("handler id is invalid")
			}

			state := state[c.HandlerID]
			if state.HandlingEvent == c.AckEvent.EventID {
				state.HandlingEvent = ""
			}

			if c.SetEnabled != nil {
				state.Enabled = *c.SetEnabled
			}

		case <-ctx.Done():
			return
		}
	}
}
