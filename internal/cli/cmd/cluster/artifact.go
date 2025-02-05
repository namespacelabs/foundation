// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/integrations/api/storage"
	"namespacelabs.dev/integrations/auth"
)

func NewArtifactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "artifact",
		Short:  "Artifact-related activities.",
		Hidden: true,
	}

	cmd.AddCommand(newArtifactUploadCmd())
	cmd.AddCommand(newArtifactDownloadCmd())

	return cmd
}

func newArtifactUploadCmd() *cobra.Command {
	var namespace string

	return fncobra.Cmd(&cobra.Command{
		Use:   "upload [src] [dest]",
		Short: "Upload an artifact.",
		Long:  "Upload an artifact. Currently, only single file uploads are supported.",
		Args:  cobra.ExactArgs(2),
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&namespace, "namespace", "", "Target namespace of the artifact.")
	}).DoWithArgs(func(ctx context.Context, args []string) error {
		if len(args) != 2 {
			return fnerrors.New("expected exactly two arguments: a local source and a remote destination")
		}
		src, dest := args[0], args[1]

		uploadFile, err := os.Open(src)
		if err != nil {
			return fnerrors.New("failed to open file %s: %w", src, err)
		}
		defer uploadFile.Close()

		token, err := auth.LoadDefaults()
		if err != nil {
			return err
		}

		cli, err := storage.NewClient(ctx, token)
		if err != nil {
			return err
		}
		defer cli.Close()

		if err := storage.UploadArtifact(ctx, cli, namespace, dest, uploadFile); err != nil {
			return err
		}

		fmt.Fprintf(console.Stdout(ctx), "Uploaded %s to %s (namespace %s)", src, dest, namespace)

		return nil
	})
}

func newArtifactDownloadCmd() *cobra.Command {
	var namespace string

	return fncobra.Cmd(&cobra.Command{
		Use:   "download [src] [dest]",
		Short: "Download an artifact.",
		Long:  "Download an artifact. Currently, only single file downloads are supported.",
		Args:  cobra.ExactArgs(2),
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&namespace, "namespace", "", "Namespace of the artifact.")
	}).DoWithArgs(func(ctx context.Context, args []string) error {
		if len(args) != 2 {
			return fnerrors.New("expected exactly two arguments: a remote source and a local destination")
		}
		src, dest := args[0], args[1]

		token, err := auth.LoadDefaults()
		if err != nil {
			return err
		}

		cli, err := storage.NewClient(ctx, token)
		if err != nil {
			return err
		}
		defer cli.Close()

		reader, err := storage.ResolveArtifactStream(ctx, cli, namespace, src)
		if err != nil {
			return err
		}
		defer reader.Close()

		w, err := os.Create(dest)
		if err != nil {
			return fnerrors.New("failed to open %q: %w", dest, err)
		}
		defer w.Close()

		if _, err := io.Copy(w, reader); err != nil {
			return fnerrors.New("failed to write %q: %w", dest, err)
		}

		fmt.Fprintf(console.Stdout(ctx), "Downloaded %s (namespace %s) to %s", src, namespace, dest)

		return nil
	})
}
