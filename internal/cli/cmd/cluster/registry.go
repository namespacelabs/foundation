// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func NewRegistryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registry",
		Short: "Registry-related activities.",
	}

	cmd.AddCommand(newShareCommand())
	return cmd
}

func newShareCommand() *cobra.Command {
	var image string

	return fncobra.Cmd(&cobra.Command{
		Use:   "share",
		Short: "Exposes a private registry image publicly.",
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&image, "image", "", "The image that should be exposed publicly.")
	}).Do(func(ctx context.Context) error {
		if image == "" {
			return fnerrors.Newf("--image is required")
		}

		ref, err := name.ParseReference(image)
		if err != nil {
			return err
		}

		repo := ref.Context().RepositoryStr()
		repoParts := strings.Split(repo, "/")

		if len(repoParts) < 2 {
			return fnerrors.Newf("repository path should contain a tenant id, please specify the whole repository path")
		}

		repoWithoutTenant := path.Join(repoParts[1:]...)
		digest := ref.Identifier()

		if !strings.HasPrefix(digest, "sha256:") {
			return fnerrors.Newf("cannot use an image with a tag, please specify a digest")
		}

		response, err := api.MakeImagePublic(ctx, api.Methods, repoWithoutTenant, digest)
		if err != nil {
			return err
		}

		fmt.Fprintf(console.Stdout(ctx), "\nThe image has been made public and can be accessed at:\n")
		fmt.Fprintf(console.Stdout(ctx), "  %s\n", response.PublicImage.PublicUrl)
		return nil
	})
}
