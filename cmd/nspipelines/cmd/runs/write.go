// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runs

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/digestfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	p "namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

func newWriteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "write",
		Args: cobra.ExactArgs(1),
	}

	flags := cmd.Flags()

	var v resultImage
	v.SetupFlags(cmd, flags)

	output := flags.String("output", "", "Where to write the image reference to.")

	_ = cmd.MarkFlagRequired("output")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		image, err := v.ComputeImage(ctx, args[0])
		if err != nil {
			return err
		}

		computed, err := compute.GetValue(ctx, image)
		if err != nil {
			return err
		}

		out, err := os.Create(*output)
		if err != nil {
			return err
		}

		r := mutate.Extract(computed)
		defer r.Close()

		_, copyErr := io.Copy(out, r)
		if copyErr != nil {
			return copyErr
		}

		return out.Close()
	})

	return cmd
}

type resultImage struct {
	Kind  string
	Label string
}

func (v *resultImage) ComputeImage(ctx context.Context, load string) (compute.Computable[oci.Image], error) {
	stored, err := protos.ReadFile[*storage.UndifferentiatedRun](load)
	if err != nil {
		return nil, err
	}

	parsedKind, ok := storage.SectionRun_Kind_value[strings.ToUpper(v.Kind)]
	if !ok {
		return nil, fnerrors.BadInputError("%s: no such kind", v.Kind)
	}

	run := &storage.SectionRun{
		Kind:        storage.SectionRun_Kind(parsedKind),
		Label:       v.Label,
		ParentRunId: stored.ParentRunId,
		Status:      stored.Status,
		Created:     stored.Created,
		Completed:   stored.Completed,
	}

	var fsys memfs.FS

	for _, r := range stored.Attachment {
		digest, err := schema.DigestOf(r.Value)
		if err != nil {
			return nil, err
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
		return nil, err
	}

	image := oci.MakeImage(oci.Scratch(), oci.MakeLayer("results", compute.Precomputed[fs.FS](&fsys, digestfs.Digest)))

	return image, nil
}

func (v *resultImage) SetupFlags(cmd *cobra.Command, flags *pflag.FlagSet) {
	flags.StringVar(&v.Kind, "kind", storage.SectionRun_KIND_UNKNOWN.String(), "The run kind.")
	flags.StringVar(&v.Label, "label", "", "A description of the run.")

	_ = cmd.MarkFlagRequired("kind")
	_ = cmd.MarkFlagRequired("label")

}
