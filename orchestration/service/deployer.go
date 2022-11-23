// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hpcloud/tail"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/framework/rpcerrors"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/runtime"
	orchpb "namespacelabs.dev/foundation/orchestration/proto"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/std/tasks/protolog"
	"namespacelabs.dev/go-ids"
)

const (
	tasksJsonFile = "tasks.json"
)

type deployer struct {
	statusDir string
	leaser    *leaser
}

type TaskEventEntry struct {
	Done      bool                                     `json:"done,omitempty"`
	Timestamp time.Time                                `json:"timestamp,omitempty"`
	Error     *spb.Status                              `json:"error,omitempty"`
	Log       *serializedMessage[*protolog.Log]        `json:"log,omitempty"`
	Event     *serializedMessage[*orchestration.Event] `json:"event,omitempty"`
}

type serializedMessage[V proto.Message] struct {
	Actual V
}

var _ json.Marshaler = &serializedMessage[proto.Message]{}
var _ json.Unmarshaler = &serializedMessage[proto.Message]{}

func newDeployer() *deployer {
	statusDir := filepath.Join(os.Getenv("NSDATA"), "status")
	if err := os.MkdirAll(statusDir, 0700|os.ModeDir); err != nil {
		log.Fatalf("unable to create dir %s: %v", statusDir, err)
	}

	return &deployer{
		statusDir: statusDir,
		leaser:    newLeaser(),
	}
}

type RunningDeployment struct {
	ID string
}

func (d *deployer) Schedule(deployPlan *schema.DeployPlan, env cfg.Context, arrival time.Time) (*RunningDeployment, error) {
	id := ids.NewRandomBase32ID(16)

	plan := execution.NewPlan(deployPlan.GetProgram().GetInvocation()...)

	dir := filepath.Join(d.statusDir, id)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("unable to create dir %s: %w", dir, err)
	}

	taskPath := filepath.Join(dir, tasksJsonFile)

	taskFile, err := os.OpenFile(taskPath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	of := &eventFile{out: taskFile}

	go func() {
		defer func() {
			if err := taskFile.Close(); err != nil {
				log.Printf("failed to close task file: %v", err)
			}
		}()

		// Use server context to not propagate context cancellation
		err := d.executeWithLog(context.Background(), of, plan, env, arrival)

		status := status.Convert(err)

		finalEvent := TaskEventEntry{
			Timestamp: time.Now(),
			Done:      true,
			Error:     status.Proto(),
		}

		if err := of.writeEvent(finalEvent); err != nil {
			log.Printf("failed to finalize task file: %v", err)
		}
	}()

	return &RunningDeployment{ID: id}, nil
}

func (d *deployer) executeWithLog(ctx context.Context, out *eventFile, plan *execution.Plan, env cfg.Context, arrival time.Time) error {
	eg := executor.New(ctx, "orchestrator.executeWithLog")

	ch := make(chan *protolog.Log)
	eg.Go(func(ctx context.Context) error {
		sink := protolog.NewSink(ch)
		defer sink.Close()

		return d.execute(tasks.WithSink(ctx, sink), out, plan, env, arrival)
	})

	eg.Go(func(ctx context.Context) error {
		return logProtos(out, ch, func(log *protolog.Log) TaskEventEntry {
			return TaskEventEntry{Log: &serializedMessage[*protolog.Log]{Actual: log}}
		})
	})

	return eg.Wait()
}

func (d *deployer) execute(ctx context.Context, out *eventFile, plan *execution.Plan, env cfg.Context, arrival time.Time) error {
	cluster, err := runtime.NamespaceFor(ctx, env)
	if err != nil {
		return err
	}

	k8s, ok := cluster.(kubedef.KubeClusterNamespace)

	var releaseLease func()
	if env.Environment().Purpose != schema.Environment_DEVELOPMENT && ok {
		// For non-dev environments, we only allow one ongoing deployment per namespace to improve robustness.
		// For dev environments, we do allow concurrent deployments to not slow down edit-refesh cycles.
		releaseLease, err = d.leaser.acquireLease(k8s.KubeConfig().Namespace, arrival)
		if err != nil {
			if err == errDeployPlanTooOld {
				// We already finished a later deployment -> skip this one.
				return nil
			}
			return err
		}
	}
	if releaseLease != nil {
		defer releaseLease()
	}

	return execution.Execute(ctx, env, "deployment.execute", plan, func(ctx context.Context) (chan *orchestration.Event, func(context.Context) error) {
		ch := make(chan *orchestration.Event)
		errCh := make(chan error)

		go func() {
			errCh <- logProtos(out, ch, func(ev *orchestration.Event) TaskEventEntry {
				return TaskEventEntry{Event: &serializedMessage[*orchestration.Event]{Actual: ev}}
			})
		}()

		return ch, func(_ context.Context) error {
			return <-errCh // Wait for the logging go-routine to return.
		}
	}, runtime.InjectCluster(cluster)...)
}

func (d *deployer) Status(ctx context.Context, id string, loglevel int32, notify func(*orchpb.DeploymentStatusResponse) error) error {
	dir := filepath.Join(d.statusDir, id)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("unknown deployment id: %s", id)
	}

	tasks, err := tail.TailFile(filepath.Join(dir, tasksJsonFile), tail.Config{
		MustExist: true,
		Follow:    true,
	})
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no task logs found for deployment id: %s", id)
		}
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case line := <-tasks.Lines:
			ev := &TaskEventEntry{}
			if err := json.Unmarshal([]byte(line.Text), ev); err != nil {
				return rpcerrors.Errorf(codes.Internal, "failed to unserialize message: %w", err)
			}

			if ev.Done {
				if ev.Error == nil {
					return nil
				}

				return status.FromProto(ev.Error).Err()
			}

			if ev.Event != nil {
				if err := notify(&orchpb.DeploymentStatusResponse{Event: ev.Event.Actual}); err != nil {
					return err
				}
			}

			if ev.Log != nil {
				log := ev.Log.Actual
				if log.LogLevel <= loglevel {
					if err := notify(&orchpb.DeploymentStatusResponse{Log: log}); err != nil {
						return err
					}
				}
			}
		}
	}
}

func (sm *serializedMessage[V]) MarshalJSON() ([]byte, error) {
	return protojson.MarshalOptions{UseProtoNames: true}.Marshal(sm.Actual)
}

func (sm *serializedMessage[V]) UnmarshalJSON(data []byte) error {
	msg := protos.NewFromType[V]()

	if err := protojson.Unmarshal(data, msg); err != nil {
		return err
	}

	sm.Actual = msg
	return nil
}

type eventFile struct {
	mu  sync.Mutex
	out *os.File
}

func (of *eventFile) writeEvent(event TaskEventEntry) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Serialize all file writes.
	of.mu.Lock()
	defer of.mu.Unlock()
	if _, err := fmt.Fprintf(of.out, "%s\n", data); err != nil {
		return err
	}

	if err := of.out.Sync(); err != nil {
		return err
	}

	return nil
}

func logProtos[V proto.Message](w *eventFile, ch chan V, makeEvent func(V) TaskEventEntry) error {
	for ev := range ch {
		event := makeEvent(ev)
		event.Timestamp = time.Now()

		if err := w.writeEvent(event); err != nil {
			// Drain the channel.
			for ev := range ch {
				log.Print("Dropped event:", ev)
			}
			return err
		}
	}

	return nil
}
