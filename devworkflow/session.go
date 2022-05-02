// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devworkflow

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/rs/zerolog"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/logoutput"
	"namespacelabs.dev/foundation/internal/syncbuffer"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/module"
	"namespacelabs.dev/foundation/workspace/tasks"
	"namespacelabs.dev/foundation/workspace/tasks/protocol"
)

var AlsoOutputToStderr = false
var AlsoOutputBuildToStderr = false
var TaskOutputBuildkitJsonLog = tasks.Output("buildkit.json", "application/json+fn.buildkit")

type SessionState struct {
	Ch chan *DevWorkflowRequest

	Console   io.Writer
	setSticky func([]byte)

	localHostname string
	obs           *Observers
	l             zerolog.Logger
	sink          *tasks.StatefulSink

	commandOutput *syncbuffer.ByteBuffer // XXX cap the size
	buildOutput   *syncbuffer.ByteBuffer // XXX cap the size
	buildkitJSON  *syncbuffer.ByteBuffer

	mu      sync.Mutex // Protect below.
	current *DevWorkflowRequest_SetWorkspace
	cancel  func()
	stack   *stackState
}

func NewStackState(ctx context.Context, sink *tasks.StatefulSink, localHostname string, stickies []string) (*SessionState, error) {
	setSticky := func(b []byte) {
		var out bytes.Buffer
		for _, sticky := range stickies {
			fmt.Fprintf(&out, " %s\n", sticky)
		}
		if len(b) > 0 && len(stickies) > 0 {
			fmt.Fprintln(&out)
			out.Write(b)
		}

		console.SetStickyContent(ctx, "stack", out.Bytes())
	}

	setSticky(nil)

	return &SessionState{
		Console:       console.TypedOutput(ctx, "fn dev", console.CatOutputUs),
		setSticky:     setSticky,
		localHostname: localHostname,
		obs:           NewObservers(ctx),
		Ch:            make(chan *DevWorkflowRequest, 1),
		l:             zerolog.Ctx(ctx).With().Logger(),
		commandOutput: syncbuffer.NewByteBuffer(),
		buildOutput:   syncbuffer.NewByteBuffer(),
		buildkitJSON:  syncbuffer.NewByteBuffer(),
		sink:          sink,
	}, nil
}

func (s *SessionState) Close(ctx context.Context) {
	close(s.Ch)
	s.obs.Close()
}

func (s *SessionState) NewClient() (chan *Update, func()) {
	ch := make(chan *Update, 1)

	const maxTaskUpload = 1000
	protos := s.sink.History(maxTaskUpload, func(t *protocol.Task) bool {
		return true
	})

	s.mu.Lock()
	// When a new client connects, send them the latest information immediately.
	// XXX keep latest computed stack in `s`.
	tu := &Update{TaskUpdate: protos}
	if s.stack != nil {
		tu.StackUpdate = s.stack.current
	}
	ch <- tu
	s.mu.Unlock()

	s.obs.Add(ch)
	return ch, func() {
		s.obs.Remove(ch)
		close(ch)
	}
}

func (s *SessionState) CommandOutput() io.ReadCloser   { return s.commandOutput.Reader() }
func (s *SessionState) BuildOutput() io.ReadCloser     { return s.buildOutput.Reader() }
func (s *SessionState) BuildJSONOutput() io.ReadCloser { return s.buildkitJSON.Reader() }

func (s *SessionState) CurrentEnv(ctx context.Context) (provision.Env, error) {
	s.mu.Lock()
	var absRoot, envName string
	if s.current != nil {
		absRoot = s.current.AbsRoot
		envName = s.current.EnvName
	}
	s.mu.Unlock()

	if absRoot == "" {
		return provision.Env{}, fnerrors.InternalError("no workspace currently configured")
	}

	root, err := module.FindRoot(ctx, absRoot)
	if err != nil {
		return provision.Env{}, err
	}

	return provision.RequireEnv(root, envName)
}

func (s *SessionState) ResolveServer(ctx context.Context, serverID string) (provision.Env, *schema.Server, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current == nil {
		return provision.Env{}, nil, fnerrors.InternalError("no current session")
	}

	root, err := module.FindRoot(ctx, s.current.AbsRoot)
	if err != nil {
		return provision.Env{}, nil, err
	}

	env, err := provision.RequireEnv(root, s.current.EnvName)
	if err != nil {
		return provision.Env{}, nil, err
	}

	if s.stack != nil {
		entry := s.stack.current.GetStack().GetServerByID(serverID)
		if entry != nil {
			return env, entry.Server, nil
		}
	}

	return provision.Env{}, nil, fnerrors.UserError(nil, "%s: no such server in the current session", serverID)
}

func (s *SessionState) handleSetWorkspace(ctx context.Context, x *DevWorkflowRequest_SetWorkspace, showLogs func(rt runtime.Runtime)) {
	s.mu.Lock()
	if s.current != nil {
		s.cancel() // Cancel whatever it is doing.
		s.cancel = nil
		s.current = nil
		s.stack = nil
	}

	if x != nil {
		newCtx, newCancel := context.WithCancel(ctx)
		s.current = x
		s.stack = &stackState{parent: s}
		s.cancel = newCancel
		do := s.stack
		s.mu.Unlock()

		// Reset the banner.
		s.setSticky(nil)

		go func() {
			err := doWorkspace(newCtx, x, do, showLogs)

			if err != nil && !errors.Is(err, context.Canceled) {
				fnerrors.Format(console.Stderr(ctx), true, err)
			}
		}()
	}
}

type sinkObserver struct{ s *SessionState }

func (so *sinkObserver) pushUpdate(ra *tasks.RunningAction) {
	p := ra.Proto()

	so.s.obs.Publish(&Update{TaskUpdate: []*protocol.Task{p}})
}

func (so *sinkObserver) OnStart(ra *tasks.RunningAction)  { so.pushUpdate(ra) }
func (so *sinkObserver) OnUpdate(ra *tasks.RunningAction) { so.pushUpdate(ra) }
func (so *sinkObserver) OnDone(ra *tasks.RunningAction)   { so.pushUpdate(ra) }

func (s *SessionState) Run(ctx context.Context, f func(ctx runtime.Runtime)) {
	cancel := s.sink.Observe(&sinkObserver{s})
	defer cancel()

	writers := []io.Writer{s.commandOutput}

	if AlsoOutputToStderr {
		writers = append(writers, console.Stderr(ctx))
	}

	var w io.Writer
	if len(writers) != 1 {
		w = io.MultiWriter(writers...)
	} else {
		w = writers[0]
	}

	ctx = logoutput.WithOutput(ctx, logoutput.OutputTo{
		Writer:     w,
		WithColors: true, // Assume xterm.js can handle color.
	})

	for {
		select {
		case <-ctx.Done():
			return

		case req := <-s.Ch:
			switch x := req.Type.(type) {
			case *DevWorkflowRequest_SetWorkspace_:
				s.handleSetWorkspace(ctx, x.SetWorkspace, f)

			case *DevWorkflowRequest_ReloadWorkspace:
				if x.ReloadWorkspace {
					s.mu.Lock()
					current := s.current
					s.mu.Unlock()
					s.handleSetWorkspace(ctx, current, f)
				}
			}
		}
	}
}

func (s *SessionState) TaskLogByName(taskID, name string) io.ReadCloser {
	return s.sink.HistoricReaderByName(taskID, name)
}

type stackState struct {
	parent  *SessionState
	current *Stack // protected by parent.mu
}

func (do *stackState) updateStack(f func(stack *Stack) *Stack) {
	do.parent.mu.Lock()
	do.current = f(do.current)
	currentCopy := proto.Clone(do.current).(*Stack)
	do.parent.mu.Unlock()
	// XXX currentCopy may be actually stale if pushUpdate is overtaken by another thread.
	do.pushUpdate(currentCopy)
}

func (do *stackState) pushUpdate(stack *Stack) {
	do.parent.obs.Publish(&Update{StackUpdate: stack})
}
