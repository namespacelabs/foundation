// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package storedrun

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/spf13/pflag"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/cli/version"
	"namespacelabs.dev/foundation/internal/codegen/protos"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/tasks"
)

var (
	OutputPath      string
	ParentID        string
	AllocatedIDPath string

	mu          sync.Mutex
	attachments []proto.Message
)

type Run struct {
	Started time.Time

	mu     sync.Mutex
	merged *storage.Command
}

type StoredRunID struct {
	RunId string `json:"run_id,omitempty"`
}

func SetupFlags(flags *pflag.FlagSet) {
	flags.StringVar(&OutputPath, "stored_run_output_path", "", "If set, outputs a serialized run to the specified path.")
	flags.StringVar(&ParentID, "stored_run_parent_id", "", "If set, tags this section with the specified push.")
	flags.StringVar(&AllocatedIDPath, "stored_run_id_path", "", "If set, uses the contents as a previously created run id.")

	_ = flags.MarkHidden("stored_run_output_path")
	_ = flags.MarkHidden("stored_run_parent_id")
	_ = flags.MarkHidden("stored_run_id_path")
}

func New() *Run {
	if OutputPath == "" {
		return nil
	}

	r := &Run{Started: time.Now(), merged: &storage.Command{}}

	if tasks.ActionStorer != nil {
		panic("tasks.ActionStorer already set?")
	}

	tasks.ActionStorer = func(ra *tasks.RunningAction) {
		cmd := tasks.Retain(ra)

		r.mu.Lock()
		r.merged.ActionLog = append(r.merged.ActionLog, cmd.ActionLog...)
		r.merged.AttachedLog = append(r.merged.AttachedLog, cmd.AttachedLog...)
		r.mu.Unlock()
	}

	return r
}

func (s *Run) Output(ctx context.Context, execErr error) error {
	s.mu.Lock()
	cmd := s.merged
	s.merged = &storage.Command{}
	s.mu.Unlock()

	Attach(cmd)

	st := nsErrorToStatus(execErr)

	run := &storage.UndifferentiatedRun{
		ParentRunId: ParentID,
		Status:      st.Proto(),
		Created:     timestamppb.New(s.Started),
		Completed:   timestamppb.Now(),
	}

	if AllocatedIDPath != "" {
		p, err := LoadStoredID(AllocatedIDPath)
		if err != nil {
			return err
		}

		run.RunId = p.RunId
	}

	attachments := consumeAttachments()

	if v, err := version.Current(); err == nil {
		attachments = append(attachments, v)
	}

	for _, attachment := range attachments {
		serialized, err := anypb.New(attachment)
		if err != nil {
			return fnerrors.InternalError("failed to serialize attachment: %w", err)
		}

		run.Attachment = append(run.Attachment, serialized)
	}

	if err := protos.WriteFile(OutputPath, run); err != nil {
		return err
	}

	return nil
}

func nsErrorToStatus(err error) *status.Status {
	st, _ := status.FromError(err)

	// Find the deepest ActionError to provide the action trace for the root cause.
	var actionErr *fnerrors.ActionError
	for {
		errors.As(err, &actionErr)
		cause := errors.Unwrap(err)
		if cause == nil {
			break
		}
		err = cause
	}

	// Extract nearest stack.
	var stackTracer fnerrors.StackTracer
	if actionErr != nil {
		errors.As(actionErr, &stackTracer)

		trace := actionErr.Trace()
		att := &storage.ActionTrace{}
		for _, a := range trace {
			ev := tasks.EventDataFromProto("", a)
			st := tasks.MakeStoreProto(&ev, nil)
			att.Task = append(att.Task, st)
		}

		if newSt, err := st.WithDetails(att); err == nil {
			st = newSt
		}
	}

	if stackTracer != nil {
		trace := stackTracer.StackTrace()
		att := &storage.StackTrace{}
		for _, f := range trace {
			st := &storage.StackTrace_Frame{
				Filename: f.File(),
				Line:     int32(f.Line()),
				Symbol:   f.Name(),
			}
			att.Frame = append(att.Frame, st)
		}
		if newSt, err := st.WithDetails(att); err == nil {
			st = newSt
		}
	}
	return st
}

func consumeAttachments() []proto.Message {
	mu.Lock()
	defer mu.Unlock()
	x := attachments
	attachments = nil
	return x
}

func Attach(m proto.Message) {
	mu.Lock()
	defer mu.Unlock()
	attachments = append(attachments, m)
}

func LoadStoredID(path string) (StoredRunID, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return StoredRunID{}, fnerrors.InternalError("failed to load run id: %w", err)
	}

	var r StoredRunID
	if err := json.Unmarshal(contents, &r); err != nil {
		return StoredRunID{}, fnerrors.InternalError("failed to load run id: %w", err)
	}

	if r.RunId == "" {
		return StoredRunID{}, fnerrors.InternalError("invalid run id")
	}

	return r, nil
}
