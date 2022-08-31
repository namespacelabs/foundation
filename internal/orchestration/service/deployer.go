// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/workspace/tasks"
	"namespacelabs.dev/foundation/workspace/tasks/simplelog"
	"namespacelabs.dev/go-ids"
)

type deployer struct {
	serverCtx context.Context

	// TODO What if there are multiple readers? Persistence!
	m  map[string]*streams
	mu sync.Mutex
}

type streams struct {
	events chan *orchestration.Event
	errch  chan error
}

func (d *deployer) Deploy(plan *schema.DeployPlan) (string, error) {
	id := ids.NewRandomBase32ID(16)

	env := makeEnv(plan)
	p := ops.NewPlan()
	if err := p.Add(plan.GetProgram().GetInvocation()...); err != nil {
		log.Printf("id %s: failed to prepare plan: %v\n", id, err)
		return "", err
	}

	errch := make(chan error, 1)
	ch := make(chan *orchestration.Event)

	d.mu.Lock()
	d.m[id] = &streams{
		events: ch,
		errch:  errch,
	}
	d.mu.Unlock()

	go func() {
		defer close(errch)

		// TODO persist logs?
		sink := simplelog.NewSink(os.Stderr, maxLogLevel)
		// Use server context to not propagate context cancellation
		ctx := tasks.WithSink(d.serverCtx, sink)

		waiters, err := p.Execute(ctx, runtime.TaskServerDeploy, env)
		if err != nil {
			log.Printf("id %s: p.Execute failed: %v\n", id, err)
			errch <- err
			return
		}

		if err := ops.WaitMultiple(ctx, waiters, ch); err != nil {
			log.Printf("id %s: ops.WaitMultiple failed: %v\n", id, err)
			errch <- err
			return
		}
	}()

	return id, nil
}

func (d *deployer) Status(id string) (*streams, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	s, ok := d.m[id]
	if !ok {
		return nil, fmt.Errorf("unknown deployment id: %s", id)
	}

	// TODO handle multiple readers!
	delete(d.m, id)
	return s, nil
}
