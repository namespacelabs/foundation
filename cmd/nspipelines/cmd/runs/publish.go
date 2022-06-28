// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runs

import (
	"archive/tar"
	"context"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/digestfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func newPublishCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "publish",
		Args: cobra.ArbitraryArgs,
	}

	flags := cmd.Flags()

	var u imageUploader
	u.SetupFlags(cmd, flags)

	insecure := flags.Bool("insecure", false, "Whether access to any specified registry is insecure.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if len(args) == 0 {
			return fnerrors.BadInputError("no inputs specified")
		}

		var images []compute.Computable[oci.Image]

		for _, arg := range args {
			if _, err := os.Stat(arg); err == nil {
				images = append(images, oci.MakeImage(oci.Scratch(), oci.LayerFromFile(os.DirFS(filepath.Dir(arg)), filepath.Base(arg))))
			} else {
				images = append(images, oci.ImageP(arg, nil, *insecure))
			}
		}

		loaded, err := compute.GetValue(ctx, compute.Collect(tasks.Action("runs.publish.load-images"), images...))
		if err != nil {
			return err
		}

		var runFS memfs.FS

		run := &storage.Run{}

		for _, l := range loaded {
			if err := oci.VisitFilesFromImage(l.Value, func(layer, path string, typ byte, contents []byte) error {
				if typ != tar.TypeReg {
					return nil
				}

				path = filepath.Clean(path)
				if path == "sectionrun.binarypb" {
					section := &storage.SectionRun{}
					if err := proto.Unmarshal(contents, section); err != nil {
						return fnerrors.BadInputError("failed to unmarshal sectionrun: %w", err)
					}
					run.SectionRun = append(run.SectionRun, section)
				} else {
					runFS.Add(path, contents)
				}

				return nil
			}); err != nil {
				return err
			}
		}

		if err := (protos.SerializeOpts{}).SerializeToFS(ctx, &runFS, map[string]proto.Message{
			"run": run,
		}); err != nil {
			return err
		}

		return u.PublishAndWrite(ctx, oci.MakeImage(oci.Scratch(), oci.MakeLayer("results", compute.Precomputed[fs.FS](&runFS, digestfs.Digest))))

	})

	return cmd
}
