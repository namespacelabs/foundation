// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package storedrun

import (
	"context"
	"sync"
	"time"

	"github.com/spf13/pflag"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

var (
	OutputPath string
	ParentID   string

	mu          sync.Mutex
	attachments []proto.Message
)

type Run struct {
	Started time.Time
}

func SetupFlags(flags *pflag.FlagSet) {
	flags.StringVar(&OutputPath, "stored_run_output_path", "", "If set, outputs a serialized run to the specified path.")
	flags.StringVar(&ParentID, "stored_run_parent_id", "", "If set, tags this section with the specified push.")

	_ = flags.MarkHidden("stored_run_output_path")
	_ = flags.MarkHidden("stored_run_parent_id")
}

func New() *Run {
	if OutputPath == "" {
		return nil
	}

	return &Run{Started: time.Now()}
}

func (s *Run) Output(ctx context.Context, execErr error) error {
	st, _ := status.FromError(execErr)

	run := &storage.UndifferentiatedRun{
		ParentRunId: ParentID,
		Status:      st.Proto(),
		Created:     timestamppb.New(s.Started),
		Completed:   timestamppb.Now(),
	}

	for _, attachment := range consumeAttachments() {
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
