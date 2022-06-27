// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package sectionrun

import (
	"sync"
	"time"

	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

var (
	ParentID string

	mu          sync.Mutex
	attachments []proto.Message
)

func Output(outputPath string, started time.Time, execErr error) error {
	mu.Lock()
	defer mu.Unlock()

	st, _ := status.FromError(execErr)

	runs := &storage.IndividualRun{
		ParentRunId: ParentID,
		Status:      st.Proto(),
		Created:     timestamppb.New(started),
		Completed:   timestamppb.Now(),
	}

	for _, attachment := range attachments {
		serialized, err := anypb.New(attachment)
		if err != nil {
			return fnerrors.InternalError("failed to serialize attachment: %w", err)
		}
		runs.Attachment = append(runs.Attachment, serialized)
	}

	if err := protos.WriteFile(outputPath, runs); err != nil {
		return err
	}

	return execErr
}

func Attach(m proto.Message) {
	mu.Lock()
	defer mu.Unlock()
	attachments = append(attachments, m)
}
