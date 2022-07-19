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

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/observers"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/runtime/endpointfwd"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/module"
	"namespacelabs.dev/foundation/workspace/tasks"
	"namespacelabs.dev/foundation/workspace/tasks/protocol"
)

type Session struct {
	requestCh chan *DevWorkflowRequest

	Errors    io.Writer
	setSticky func(string)

	executor      *executor.Executor
	localHostname string
	obs           *Observers
	sink          *tasks.StatefulSink

	mu        sync.Mutex // Protect below.
	requested struct {
		absRoot string
		envName string
		servers []string
	}
	cancelWorkspace func()
	currentStack    *Stack
	currentEnv      runtime.Selector
	pfw             *endpointfwd.PortForward
}

func NewSession(ctx context.Context, sink *tasks.StatefulSink, localHostname string) (*Session, error) {
	setSticky := func(b string) {
		console.SetStickyContent(ctx, "stack", b)
	}

	return &Session{
		requestCh:     make(chan *DevWorkflowRequest, 1),
		Errors:        console.Errors(ctx),
		setSticky:     setSticky,
		executor:      executor.New(ctx, "devworkflow.session"),
		localHostname: localHostname,
		obs:           NewObservers(ctx),
		sink:          sink,
	}, nil
}

func (s *Session) DeferRequest(req *DevWorkflowRequest) {
	// XXX check that the session is not done.
	s.requestCh <- req
}

func (s *Session) NewClient(needsHistory bool) (*Observer, error) {
	const maxTaskUpload = 1000
	var taskHistory []*protocol.Task

	if needsHistory {
		taskHistory = s.sink.History(maxTaskUpload, func(t *protocol.Task) bool {
			return true
		})
	}

	s.mu.Lock()
	// When a new client connects, send them the latest information immediately.
	// XXX keep latest computed stack in `s`.
	tu := &Update{TaskUpdate: taskHistory, StackUpdate: protos.Clone(s.currentStack)}
	s.mu.Unlock()

	return s.obs.New(tu, false)
}

// Implements observers.SessionProvider.
func (s *Session) NewStackClient() (observers.StackSession, error) {
	s.mu.Lock()
	tu := &Update{StackUpdate: protos.Clone(s.currentStack)}
	s.mu.Unlock()

	return s.obs.New(tu, true)
}

// XXX these need to be re-implemented.
func (s *Session) CommandOutput() io.ReadCloser   { return io.NopCloser(bytes.NewReader(nil)) }
func (s *Session) BuildOutput() io.ReadCloser     { return io.NopCloser(bytes.NewReader(nil)) }
func (s *Session) BuildJSONOutput() io.ReadCloser { return io.NopCloser(bytes.NewReader(nil)) }

func (s *Session) ResolveServer(ctx context.Context, serverID string) (runtime.Selector, *schema.Server, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := s.currentStack.GetStack().GetServerByID(serverID)
	if entry != nil {
		return s.currentEnv, entry.Server, nil
	}

	return nil, nil, fnerrors.UserError(nil, "%s: no such server in the current session", serverID)
}

func (s *Session) handleSetWorkspace(parentCtx context.Context, absRoot, envName string, servers []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancelWorkspace != nil {
		s.cancelWorkspace() // Cancel whatever it is doing.
		s.cancelWorkspace = nil
	}

	previousPortFwds := s.currentStack.GetForwardedPort()
	s.currentStack = &Stack{ForwardedPort: previousPortFwds}

	s.requested.absRoot = absRoot
	s.requested.envName = envName
	s.requested.servers = servers

	fmt.Fprintf(console.Debug(parentCtx), "devworkflow: setWorkspace: %s %s %v\n", envName, absRoot, servers)

	if len(servers) > 0 {
		ctx, newCancel := context.WithCancel(parentCtx)
		s.cancelWorkspace = newCancel

		// Reset the banner.
		s.setSticky("")

		env, err := loadWorkspace(ctx, absRoot, envName)
		if err != nil {
			s.cancelPortForward()
			return err
		}

		resetStack(s.currentStack, env, nil)
		pfw := s.setEnvironment(parentCtx, env)

		s.executor.Go(func(ctx context.Context) error {
			err := setWorkspace(ctx, env, servers[0], servers[1:], s, pfw)

			if err != nil && !errors.Is(err, context.Canceled) {
				fnerrors.Format(console.Stderr(parentCtx), err, fnerrors.WithStyle(colors.WithColors))
			}

			return err
		})
	}

	return nil
}

func loadWorkspace(ctx context.Context, absRoot, envName string) (provision.Env, error) {
	// Re-create loc/root here, to dump the cache.
	root, err := module.FindRoot(ctx, absRoot)
	if err != nil {
		return provision.Env{}, err
	}

	return provision.RequireEnv(root, envName)
}

type sinkObserver struct{ s *Session }

func (so *sinkObserver) pushUpdate(ra *tasks.RunningAction) {
	p := ra.Proto()

	so.s.obs.Publish(&Update{TaskUpdate: []*protocol.Task{p}})
}

func (so *sinkObserver) OnStart(ra *tasks.RunningAction)  { so.pushUpdate(ra) }
func (so *sinkObserver) OnUpdate(ra *tasks.RunningAction) { so.pushUpdate(ra) }
func (so *sinkObserver) OnDone(ra *tasks.RunningAction)   { so.pushUpdate(ra) }

func (s *Session) Run(ctx context.Context) error {
	defer s.obs.Close()

	cancel := s.sink.Observe(&sinkObserver{s})
	defer cancel()

	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.cancelPortForward()
	}()

	defer close(s.requestCh)

	s.executor.Go(func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()

			case req := <-s.requestCh:
				switch x := req.Type.(type) {
				case *DevWorkflowRequest_SetWorkspace_:
					set := x.SetWorkspace
					servers := append([]string{set.GetPackageName()}, set.GetAdditionalServers()...)

					s.executor.Go(func(ctx context.Context) error {
						return s.handleSetWorkspace(ctx, set.GetAbsRoot(), set.GetEnvName(), servers)
					})

				case *DevWorkflowRequest_ReloadWorkspace:
					if x.ReloadWorkspace {
						s.mu.Lock()
						absRoot := s.requested.absRoot
						envName := s.requested.envName
						servers := s.requested.servers
						s.mu.Unlock()

						s.executor.Go(func(ctx context.Context) error {
							return s.handleSetWorkspace(ctx, absRoot, envName, servers)
						})
					}
				}
			}
		}
	})

	return s.executor.Wait()
}

func (s *Session) RunInContext(f func(context.Context) error) {
	s.executor.Go(f)
}

func (s *Session) TaskLogByName(taskID, name string) io.ReadCloser {
	return s.sink.HistoricReaderByName(tasks.ActionID(taskID), name)
}

func (s *Session) setEnvironment(parentCtx context.Context, env runtime.Selector) *endpointfwd.PortForward {
	if s.pfw != nil && proto.Equal(s.currentEnv.Proto(), env.Proto()) {
		// Nothing to do.
		return s.pfw
	}

	s.cancelPortForward()

	s.pfw = NewPortFwd(parentCtx, s, env, s.localHostname)
	s.currentEnv = env
	return s.pfw
}

func (s *Session) cancelPortForward() {
	if s.pfw != nil {
		if err := s.pfw.Cleanup(); err != nil {
			fmt.Fprintln(s.Errors, "Failed to cleanup port forwarding resources", err)
		}
		s.pfw = nil
	}
}

func (s *Session) updateStackInPlace(f func(stack *Stack)) {
	s.mu.Lock()
	f(s.currentStack)
	s.currentStack.RenderedPortForwarding = s.pfw.Render(colors.WithColors)
	s.currentStack.Revision++
	copy := protos.Clone(s.currentStack)
	s.mu.Unlock()

	s.obs.Publish(&Update{StackUpdate: copy})
}
