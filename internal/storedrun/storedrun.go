// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package storedrun

import (
	"context"
	"fmt"
	"io/fs"
	"sync"
	"time"

	"github.com/spf13/pflag"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/digestfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	p "namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

var (
	OutputPath                 string
	ParentID                   string
	UploadToRegistryRepository string

	mu          sync.Mutex
	attachments []proto.Message
)

type Run struct {
	Started time.Time
}

func SetupFlags(flags *pflag.FlagSet) {
	flags.StringVar(&OutputPath, "stored_run_output_path", "", "If set, outputs a serialized run to the specified path.")
	flags.StringVar(&ParentID, "stored_run_parent_id", "", "If set, tags this section with the specified push.")
	flags.StringVar(&UploadToRegistryRepository, "stored_run_upload_to_repository", "", "If set, uploads the serialize run into the a repository in the environment bound's registry.")

	flags.MarkHidden("stored_run_output_path")
	flags.MarkHidden("stored_run_parent_id")
	flags.MarkHidden("stored_run_upload_to_repository")
}

func Check() (*Run, error) {
	if OutputPath == "" {
		return nil, nil
	}

	return &Run{Started: time.Now()}, nil
}

func (s *Run) Output(ctx context.Context, env ops.Environment, execErr error) error {
	if s == nil {
		return execErr
	}

	st, _ := status.FromError(execErr)

	run := &storage.IndividualRun{
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

	stored := &storage.StoredIndividualRun{
		Run: run,
	}

	if UploadToRegistryRepository != "" {
		tag, err := registry.RawAllocateName(ctx, devhost.ConfigKeyFromEnvironment(env), UploadToRegistryRepository)
		if err != nil {
			return fnerrors.InternalError("failed to allocate image for stored results: %w", err)
		}

		tag2, _ := compute.GetValue(ctx, tag)
		fmt.Fprintf(console.Debug(ctx), "tag: %+v (from %s)\n", tag2, UploadToRegistryRepository)

		var toFS memfs.FS

		if err := (p.SerializeOpts{}).SerializeToFS(ctx, &toFS, map[string]proto.Message{
			"run": run,
		}); err != nil {
			return fnerrors.InternalError("serializing stored results failed: %w", err)
		}

		imageID, err := compute.GetValue(ctx, oci.PublishImage(tag, oci.MakeImage(oci.Scratch(), oci.MakeLayer("results", compute.Precomputed[fs.FS](&toFS, digestfs.Digest)))))
		if err != nil {
			return fnerrors.InternalError("failed to store results: %w", err)
		}

		stored.StoredRunId = imageID.ImageRef()
	}

	if err := protos.WriteFile(OutputPath, stored); err != nil {
		return err
	}

	return execErr
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
