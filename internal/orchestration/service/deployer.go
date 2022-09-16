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
	"time"

	"github.com/hpcloud/tail"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/orchestration/proto"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/tasks"
	"namespacelabs.dev/foundation/workspace/tasks/protolog"
	"namespacelabs.dev/go-ids"
)

const (
	eof           = "EOF" // magic marker to signal when to stop tailing logs
	taskFile      = "tasks.json"
	eventFile     = "events.json"
	errFile       = "error.txt"
	updateTimeout = time.Minute // Deployments can take very long (e.g. > 15 minutes for RDS cluster setup) but we expect regular log/event updates.
)

type deployer struct {
	statusDir string
	leaser    *leaser
}

func newDeployer() deployer {
	statusDir := filepath.Join(os.Getenv("NSDATA"), "status")
	if err := os.MkdirAll(statusDir, 0700|os.ModeDir); err != nil {
		panic(fmt.Sprintf("unable to create dir %s: %v", statusDir, err))
	}

	return deployer{
		statusDir: statusDir,
		leaser:    newLeaser(),
	}
}

type RunningDeployment struct {
	ID string
}

func (d *deployer) Schedule(plan *schema.DeployPlan, env planning.Context, arrival time.Time) (*RunningDeployment, error) {
	id := ids.NewRandomBase32ID(16)

	p, err := ops.NewPlan(plan.GetProgram().GetInvocation()...)
	if err != nil {
		log.Printf("id %s: failed to prepare plan: %v\n", id, err)
		return nil, err
	}

	dir := filepath.Join(d.statusDir, id)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("unable to create dir %s: %w", dir, err)
	}

	eventPath := filepath.Join(dir, eventFile)
	taskPath := filepath.Join(dir, taskFile)

	if err := ensureFiles(eventPath, taskPath); err != nil {
		return nil, err
	}

	go func() {
		// We only close files here to ensure
		// 1) errors (if any) have been persisted already
		// 2) files end with eof - no matter what
		defer func() {
			markEof(eventPath)
			markEof(taskPath)
		}()

		// Use server context to not propagate context cancellation
		if err := d.executeWithLog(context.Background(), eventPath, taskPath, p, env, arrival); err != nil {
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

	return &RunningDeployment{ID: id}, nil
}

func (d *deployer) executeWithLog(ctx context.Context, eventPath, taskPath string, p *ops.Plan, env planning.Context, arrival time.Time) error {
	eg := executor.New(ctx, "orchestrator.executeWithLog")

	ch := make(chan *protolog.Log)
	eg.Go(func(ctx context.Context) error {
		sink := protolog.NewSink(ch)
		defer sink.Close()

		return d.execute(tasks.WithSink(ctx, sink), eventPath, p, env, arrival)
	})

	eg.Go(func(ctx context.Context) error {
		return logProtos(taskPath, ch)
	})

	return eg.Wait()
}

func (d *deployer) execute(ctx context.Context, eventPath string, p *ops.Plan, env planning.Context, arrival time.Time) error {
	cluster, err := runtime.NamespaceFor(ctx, env)
	if err != nil {
		return err
	}

	ns := cluster.Planner().Namespace()

	releaseLease, err := d.leaser.acquireLease(ns.UniqueID(), arrival)
	if err != nil {
		if err == errDeployPlanTooOld {
			// We already finished a later deployment -> skip this one.
			return nil
		}
		return err
	}
	defer releaseLease()

	return ops.Execute(ctx, env, "deployment.execute", p, func(ctx context.Context) (chan *orchestration.Event, func(error) error) {
		ch := make(chan *orchestration.Event)

		logErrCh := make(chan error)

		go func() {
			logErrCh <- logProtos(eventPath, ch)
		}()

		return ch, func(err error) error {
			logErr := <-logErrCh // Wait for the logging go-routine to return.
			if err != nil {
				return err
			}
			return logErr
		}
	}, runtime.InjectCluster(cluster)...)
}

func logProtos[V any](filename string, ch chan *V) error {
	for ev := range ch {
		data, err := json.Marshal(ev)
		if err != nil {
			return err
		}

		if err := appendLine(filename, string(data)); err != nil {
			return err
		}
	}

	return nil
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

func markEof(path string) {
	if err := appendLine(path, eof); err != nil {
		log.Printf("Unable to append to file %s: %v", path, err)
	}
}

func ensureFiles(paths ...string) error {
	for _, path := range paths {
		if err := os.WriteFile(path, nil, 0644); err != nil {
			return err
		}
	}
	return nil
}

func (d *deployer) Status(ctx context.Context, id string, loglevel int32, notify func(*proto.DeploymentStatusResponse) error) error {
	dir := filepath.Join(d.statusDir, id)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("unknown deployment id: %s", id)
	}

	events, err := tail.TailFile(filepath.Join(dir, eventFile), tail.Config{
		MustExist: true,
		Follow:    true,
	})
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no events found for deployment id: %s", id)
		}
		return err
	}

	tasks, err := tail.TailFile(filepath.Join(dir, taskFile), tail.Config{
		MustExist: true,
		Follow:    true,
	})
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no task logs found for deployment id: %s", id)
		}
		return err
	}

	var tasksDone, eventsDone bool
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-time.After(updateTimeout):
			return fmt.Errorf("deployment %s likely died: didn't receive any event/log update in %v", id, updateTimeout)

		case line := <-events.Lines:
			if line.Text == eof {
				eventsDone = true
			} else {
				ev := &orchestration.Event{}
				if err := json.Unmarshal([]byte(line.Text), ev); err != nil {
					return err
				}

				if err := notify(&proto.DeploymentStatusResponse{Event: ev}); err != nil {
					return err
				}
			}

		case line := <-tasks.Lines:
			if line.Text == eof {
				tasksDone = true
			} else {
				log := &protolog.Log{}
				if err := json.Unmarshal([]byte(line.Text), log); err != nil {
					return err
				}

				if log.LogLevel <= loglevel {
					if err := notify(&proto.DeploymentStatusResponse{Log: log}); err != nil {
						return err
					}
				}
			}
		}

		if tasksDone && eventsDone {
			break
		}
	}

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
