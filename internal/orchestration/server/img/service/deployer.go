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

	"github.com/hpcloud/tail"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/tasks"
	"namespacelabs.dev/foundation/workspace/tasks/simplelog"
	"namespacelabs.dev/go-ids"
)

const (
	maxLogLevel = 0
	eof         = "EOF" // magic marker to signal when to stop tailing logs
	eventFile   = "events.json"
	errFile     = "error.txt"
)

type deployer struct {
	serverCtx context.Context
	statusDir string
}

func makeDeployer(ctx context.Context) deployer {
	statusDir := filepath.Join(os.Getenv("NSDATA"), "status")
	if err := os.MkdirAll(statusDir, 0700|os.ModeDir); err != nil {
		panic(fmt.Sprintf("unable to create dir %s: %v", statusDir, err))
	}

	return deployer{
		serverCtx: ctx,
		statusDir: statusDir,
	}
}

func (d *deployer) Schedule(plan *schema.DeployPlan, env *env) (string, error) {
	id := ids.NewRandomBase32ID(16)

	p := ops.NewPlan()
	if err := p.Add(plan.GetProgram().GetInvocation()...); err != nil {
		log.Printf("id %s: failed to prepare plan: %v\n", id, err)
		return "", err
	}

	dir := filepath.Join(d.statusDir, id)
	if err := os.MkdirAll(dir, 0700|os.ModeDir); err != nil {
		return "", fmt.Errorf("unable to create dir %s: %w", dir, err)
	}

	// Ensure event log exists
	eventPath := filepath.Join(dir, eventFile)
	if f, err := os.OpenFile(eventPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err != nil {
		return "", err
	} else {
		f.Close()
	}

	go func() {
		defer func() {
			// Indicate end of event stream
			if err := appendLine(eventPath, eof); err != nil {
				log.Printf("Unable to append to file %s: %v", eventPath, err)
			}
		}()

		// Use server context to not propagate context cancellation
		if err := execute(d.serverCtx, eventPath, p, env); err != nil {
			status := status.Convert(err)
			data, jsonErr := json.Marshal(status.Proto())
			if jsonErr != nil {
				log.Printf("Unable to marshal error %v:\n%v", err, jsonErr)
				return
			}
			errPath := filepath.Join(dir, errFile)
			if err := appendLine(errPath, string(data)); err != nil {
				log.Printf("Unable to append to file %s: %v", errPath, err)
			}
		}
	}()

	return id, nil
}

func execute(ctx context.Context, eventPath string, p *ops.Plan, env planning.Context) error {
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
		errch <- logEvents(eventPath, ch)
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

	dir := filepath.Join(d.statusDir, id)

	t, err := tail.TailFile(filepath.Join(dir, eventFile), tail.Config{
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
				data, err := os.ReadFile(filepath.Join(dir, errFile))
				if err != nil {
					if os.IsNotExist(err) {
						// no deployment error found
						return nil
					}
					return fmt.Errorf("unable to read deployment error: %w", err)
				}

				proto := &spb.Status{}
				if err := json.Unmarshal(data, proto); err != nil {
					return fmt.Errorf("unable to unmarshal deployment error: %w", err)
				}
				return status.FromProto(proto).Err()
			}

			ev := &orchestration.Event{}
			if err := json.Unmarshal([]byte(line.Text), ev); err != nil {
				return err
			}

			ch <- ev
		}
	}
}
