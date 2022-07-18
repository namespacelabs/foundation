// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runs

import (
	"archive/tar"
	"context"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnapi"
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

	runID := flags.String("run_id", "", "The parent run id.")
	sourceRepo := flags.String("source_repo", "", "The repository we pushed from.")
	commitID := flags.String("source_commit", "", "The commit we pushed from.")
	sourceBranch := flags.String("source_branch", "", "The branch we pushed from.")
	pullRequest := flags.String("source_pull_request", "", "The pull request we pushed from.")
	author := flags.String("author", "", "The username of author of the commit.")
	moduleName := flags.String("module_name", "", "The module that was just worked on.")
	githubEvent := flags.String("github_event_path", "", "Path to a file with github's event json.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if len(args) == 0 {
			return fnerrors.BadInputError("no inputs specified")
		}

		var images []compute.Computable[oci.Image]

		for _, arg := range args {
			if _, err := os.Stat(arg); err == nil {
				images = append(images, oci.MakeImageFromScratch(arg, oci.LayerFromFile(arg, os.DirFS(filepath.Dir(arg)), filepath.Base(arg))).Image())
			} else {
				images = append(images, oci.ImageP(arg, nil, oci.ResolveOpts{InsecureRegistry: *insecure}))
			}
		}

		loaded, err := compute.GetValue(ctx, compute.Collect(tasks.Action("runs.publish.load-images"), images...))
		if err != nil {
			return err
		}

		var runFS memfs.FS

		run := &storage.Run{
			RunId:       *runID,
			Repository:  *sourceRepo,
			CommitId:    *commitID,
			Branch:      *sourceBranch,
			PullRequest: *pullRequest,
			AuthorLogin: *author,
			ModuleName:  *moduleName,
		}

		if *githubEvent != "" {
			contents, err := ioutil.ReadFile(*githubEvent)
			if err != nil {
				return fnerrors.InternalError("failed to load %q: %w", *githubEvent, err)
			}

			serialized, err := anypb.New(&storage.GithubEvent{
				SerializedJson: string(contents),
			})
			if err != nil {
				return fnerrors.InternalError("failed to serialize json: %w", err)
			}

			run.Attachment = append(run.Attachment, serialized)
		}

		userAuth, err := fnapi.LoadUser()
		if err == nil {
			run.PusherLogin = userAuth.Username
		}

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

		return u.PublishAndWrite(ctx, oci.MakeImageFromScratch("run", oci.MakeLayer("run", compute.Precomputed[fs.FS](&runFS, digestfs.Digest))))

	})

	return cmd
}
