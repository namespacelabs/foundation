// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/hpcloud/tail"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/providers/aws"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/workspace/tasks"
	"namespacelabs.dev/foundation/workspace/tasks/simplelog"
	"namespacelabs.dev/go-ids"
)

const (
	maxLogLevel = 0
	eof         = "EOF" // magic marker to signal when to stop tailing logs
)

type deployer struct {
	serverCtx context.Context
	eventDir  string

	// TODO write to PVC, too?
	errors map[string]error // protected by mu
	mu     sync.RWMutex
}

func makeDeployer(ctx context.Context) deployer {
	eventDir := filepath.Join(os.Getenv("NSDATA"), "events")
	if err := os.MkdirAll(eventDir, 0700|os.ModeDir); err != nil {
		panic(fmt.Sprintf("unable to create dir %s: %v", eventDir, err))
	}

	return deployer{
		serverCtx: ctx,
		eventDir:  eventDir,
		errors:    make(map[string]error),
	}
}

func (d *deployer) Schedule(plan *schema.DeployPlan, awsConf *aws.Conf) (string, error) {
	id := ids.NewRandomBase32ID(16)

	env := makeEnv(plan, awsConf)
	p := ops.NewPlan()
	if err := p.Add(plan.GetProgram().GetInvocation()...); err != nil {
		log.Printf("id %s: failed to prepare plan: %v\n", id, err)
		return "", err
	}

	// Ensure event log exists
	filepath := d.eventPath(id)
	if f, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err != nil {
		return "", err
	} else {
		f.Close()
	}

	go func() {
		defer func() {
			// Indicate end of event stream
			if err := appendLine(filepath, eof); err != nil {
				log.Printf("Unable to append to file %s: %v", filepath, err)
			}
		}()

		// Use server context to not propagate context cancellation
		if err := d.execute(d.serverCtx, id, p, env); err != nil {
			d.mu.Lock()
			defer d.mu.Unlock()
			d.errors[id] = err
		}
	}()

	return id, nil
}

func (d *deployer) eventPath(id string) string {
	return filepath.Join(d.eventDir, fmt.Sprintf("%s.json", id))
}

func (d *deployer) execute(ctx context.Context, id string, p *ops.Plan, env ops.Environment) error {
	// TODO persist logs?
	sink := simplelog.NewSink(os.Stderr, maxLogLevel)
	ctx = tasks.WithSink(ctx, sink)

	waiters, err := p.Execute(ctx, runtime.TaskServerDeploy, env)
	if err != nil {
		return err
	}

	errch := make(chan error, 1)
	ch := make(chan *orchestration.Event)
	go func() {
		defer close(errch)
		errch <- logEvents(d.eventPath(id), ch)
	}()

	waitErr := ops.WaitMultiple(ctx, waiters, ch)
	logErr := <-errch

	return multierr.New(waitErr, logErr)
}

func logEvents(filename string, ch chan *orchestration.Event) error {
	for {
		ev, ok := <-ch
		if !ok {
			return nil
		}

		data, err := json.Marshal(ev)
		if err != nil {
			return err
		}

		if err := appendLine(filename, string(data)); err != nil {
			return err
		}
	}
}

func appendLine(filename, line string) error {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString(fmt.Sprintf("%s\n", line)); err != nil {
		return err
	}

	return nil
}

func (d *deployer) Status(ctx context.Context, id string, ch chan *orchestration.Event) error {
	defer close(ch)

	filename := d.eventPath(id)

	t, err := tail.TailFile(filename, tail.Config{
		MustExist: true,
		Follow:    true,
	})
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("unknown deployment id: %s", id)
		}
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case line := <-t.Lines:
			if line.Text == eof {
				d.mu.RLock()
				defer d.mu.RUnlock()

				if err, ok := d.errors[id]; ok {
					return err
				}
				return nil
			}

			ev := &orchestration.Event{}
			if err := json.Unmarshal([]byte(line.Text), ev); err != nil {
				return err
			}

			ch <- ev
		}
	}
}
