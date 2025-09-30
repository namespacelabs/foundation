// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/aws/smithy-go/ptr"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func NewImageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Image related activities.",
	}

	cmd.AddCommand(newShareCommand())
	return cmd
}

func newShareCommand() *cobra.Command {
	var expirationDur time.Duration

	return fncobra.Cmd(&cobra.Command{
		Use:   "share [image] { --expires_in=[duration] }",
		Short: "Exposes an image publicly with an optional expiration.",
		Args:  cobra.ExactArgs(1),
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.DurationVar(&expirationDur, "expires_in", 0, "If set, the public image share expires in the future.")
	}).DoWithArgs(func(ctx context.Context, args []string) error {
		if len(args) != 1 {
			return fnerrors.Newf("expected exactly one arguments: an image reference needs to be specified")
		}

		image := args[0]
		ref, err := name.ParseReference(image)
		if err != nil {
			return err
		}

		registry := ref.Context().RegistryStr()

		if !strings.HasSuffix(registry, "nscr.io") {
			return fnerrors.Newf("can only make nscr.io registry images public")
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

		req := api.MakeImagePublicRequest{
			Repository: repoWithoutTenant,
			Digest:     digest,
		}

		if expirationDur > 0 {
			req.ExpiresAt = ptr.Time(time.Now().Add(expirationDur))
		} else if expirationDur < 0 {
			return fnerrors.BadInputError("expiration cannot be negative")
		}

		response, err := api.MakeImagePublic(ctx, api.Methods, req)
		if err != nil {
			return err
		}

		fmt.Fprintf(console.Stdout(ctx), "\nThe image has been made public and can be accessed at:\n")
		fmt.Fprintf(console.Stdout(ctx), "  %s\n", response.PublicImage.PublicUrl)
		return nil
	})
}
