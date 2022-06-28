// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runs

import (
	"context"
	"io/fs"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/digestfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	p "namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/module"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

func newUploadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "upload",
		Args: cobra.ExactArgs(1),
	}

	flags := cmd.Flags()

	envName := flags.String("env", "dev", "The environment to load configuration from.")
	kind := flags.String("kind", storage.SectionRun_KIND_UNKNOWN.String(), "The run kind.")
	label := flags.String("label", "", "A description of the run.")
	workspacePath := flags.String("workspace", "", "The workspace where to load configuration from.")
	output := flags.String("output", "", "Where to write the image reference to.")
	repository := flags.String("repository", "", "The repository to upload to.")

	_ = cmd.MarkFlagRequired("kind")
	_ = cmd.MarkFlagRequired("label")
	_ = cmd.MarkFlagRequired("workspace")
	_ = cmd.MarkFlagRequired("output")
	_ = cmd.MarkFlagRequired("repository")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		devhost.HasRuntime = runtime.HasRuntime

		workspace.ModuleLoader = cuefrontend.ModuleLoader
		workspace.MakeFrontend = cuefrontend.NewFrontend

		root, err := module.FindRoot(ctx, *workspacePath)
		if err != nil {
			return err
		}

		env, err := provision.RequireEnv(root, *envName)
		if err != nil {
			return err
		}

		stored, err := protos.ReadFile[*storage.UndifferentiatedRun](args[0])
		if err != nil {
			return err
		}

		parsedKind, ok := storage.SectionRun_Kind_value[strings.ToUpper(*kind)]
		if !ok {
			return fnerrors.BadInputError("%s: no such kind", *kind)
		}

		run := &storage.SectionRun{
			Kind:        storage.SectionRun_Kind(parsedKind),
			Label:       *label,
			ParentRunId: stored.ParentRunId,
			Status:      stored.Status,
			Created:     stored.Created,
			Completed:   stored.Completed,
		}

		var fsys memfs.FS

		for _, r := range stored.Attachment {
			digest, err := schema.DigestOf(r.Value)
			if err != nil {
				return err
			}

			p := filepath.Join("blobs", digest.String())

			fsys.Add(p, r.Value)

			run.StoredAttachment = append(run.StoredAttachment, &storage.SectionRun_StoredAttachment{
				TypeUrl:   r.TypeUrl,
				ImagePath: p,
			})
		}

		if err := (p.SerializeOpts{}).SerializeToFS(ctx, &fsys, map[string]proto.Message{
			"sectionrun": run,
		}); err != nil {
			return err
		}

		tag, err := registry.RawAllocateName(ctx, devhost.ConfigKeyFromEnvironment(env), *repository)
		if err != nil {
			return fnerrors.InternalError("failed to allocate image for stored results: %w", err)
		}

		imageID, err := compute.GetValue(ctx, oci.PublishImage(tag, oci.MakeImage(oci.Scratch(), oci.MakeLayer("results", compute.Precomputed[fs.FS](&fsys, digestfs.Digest)))))
		if err != nil {
			return fnerrors.InternalError("failed to store results: %w", err)
		}

		return ioutil.WriteFile(*output, []byte(imageID.ImageRef()), 0644)
	})

	return cmd
}
