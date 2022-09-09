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
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
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
	eof       = "EOF" // magic marker to signal when to stop tailing logs
	taskFile  = "tasks.json"
	eventFile = "events.json"
	errFile   = "error.txt"
)

type deployer struct {
	serverCtx context.Context
	statusDir string
	leaser    *leaser
}

func makeDeployer(ctx context.Context) deployer {
	statusDir := filepath.Join(os.Getenv("NSDATA"), "status")
	if err := os.MkdirAll(statusDir, 0700|os.ModeDir); err != nil {
		panic(fmt.Sprintf("unable to create dir %s: %v", statusDir, err))
	}

	return deployer{
		serverCtx: ctx,
		statusDir: statusDir,
		leaser:    newLeaser(),
	}
}

func (d *deployer) Schedule(plan *schema.DeployPlan, env planning.Context, arrival time.Time) (string, error) {
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

	eventPath := filepath.Join(dir, eventFile)
	if err := ensureFile(eventPath); err != nil {
		return "", err
	}

	taskPath := filepath.Join(dir, taskFile)
	if err := ensureFile(taskPath); err != nil {
		return "", err
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
		if err := d.executeWithLog(eventPath, taskPath, p, env, arrival); err != nil {
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

func (d *deployer) executeWithLog(eventPath, taskPath string, p *ops.Plan, env planning.Context, arrival time.Time) error {
	ch := make(chan *protolog.Log)

	errch := make(chan error, 1)
	go func() {
		sink := protolog.NewSink(ch)
		defer sink.Close()

		ctx := tasks.WithSink(d.serverCtx, sink)
		errch <- d.execute(ctx, eventPath, p, env, arrival)
	}()

	logErr := logProtos(taskPath, ch)
	execErr := <-errch

	return multierr.New(execErr, logErr)
}

func (d *deployer) execute(ctx context.Context, eventPath string, p *ops.Plan, env planning.Context, arrival time.Time) error {
	cluster, err := runtime.ClusterFor(ctx, env)
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

	// Make sure that the cluster is accessible to a serialized invocation implementation.
	ctx = runtime.ClusterInjection.With(ctx, cluster)

	waiters, err := ops.Execute(ctx, runtime.TaskServerDeploy, env, p)
	if err != nil {
		return err
	}

	errch := make(chan error, 1)
	ch := make(chan *orchestration.Event)
	go func() {
		defer close(errch)
		errch <- ops.WaitMultiple(ctx, waiters, ch)
	}()

	logErr := logProtos(eventPath, ch)
	waitErr := <-errch

	return multierr.New(waitErr, logErr)
}

func logProtos[V any](filename string, ch chan *V) error {
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

func markEof(path string) {
	if err := appendLine(path, eof); err != nil {
		log.Printf("Unable to append to file %s: %v", path, err)
	}
}

func ensureFile(path string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	return nil
}

func (d *deployer) Status(ctx context.Context, id string, loglevel int32, ch chan *proto.DeploymentStatusResponse) error {
	defer close(ch)

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
		case line := <-events.Lines:
			if line.Text == eof {
				eventsDone = true
			} else {
				ev := &orchestration.Event{}
				if err := json.Unmarshal([]byte(line.Text), ev); err != nil {
					return err
				}

				ch <- &proto.DeploymentStatusResponse{Event: ev}
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
					ch <- &proto.DeploymentStatusResponse{Log: log}
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
