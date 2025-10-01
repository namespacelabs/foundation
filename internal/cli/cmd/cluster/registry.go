// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	registryv1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/registry/v1beta"
	"buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/stdlib"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnerrors"
	registryapi "namespacelabs.dev/integrations/api/registry"
	"namespacelabs.dev/integrations/auth"
)

func NewRegistryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registry",
		Short: "Manage container registry images.",
	}

	cmd.AddCommand(newRegistryListCmd())
	cmd.AddCommand(newRegistryDescribeCmd())
	cmd.AddCommand(newRegistryShareCmd())

	return cmd
}

func newRegistryListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List images or repositories in the registry.",
		Args:  cobra.NoArgs,
	}

	repositories := cmd.Flags().Bool("repositories", false, "List repositories instead of images")
	output := cmd.Flags().StringP("output", "o", "table", "Output format: table, json")
	matchRepo := cmd.Flags().String("repository", "", "Filter images by repository name")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		tokenSource, err := auth.LoadDefaults()
		if err != nil {
			return fnerrors.InvocationError("registry", "failed to get authentication token: %w", err)
		}

		client, err := registryapi.NewClient(ctx, tokenSource)
		if err != nil {
			return fnerrors.InvocationError("registry", "failed to create registry client: %w", err)
		}
		defer client.Close()

		if *repositories {
			resp, err := client.ContainerRegistry.ListRepositories(ctx, &registryv1beta.ListRepositoriesRequest{})
			if err != nil {
				return fnerrors.InvocationError("registry", "failed to list repositories: %w", err)
			}

			switch *output {
			case "json":
				var b bytes.Buffer
				fmt.Fprint(&b, "[")
				for k, repo := range resp.Repositories {
					if k > 0 {
						fmt.Fprint(&b, ",")
					}

					bb, err := protojson.MarshalOptions{UseProtoNames: true, Multiline: true}.Marshal(repo)
					if err != nil {
						return err
					}

					fmt.Fprintf(&b, "\n%s", bb)
				}
				fmt.Fprint(&b, "\n]\n")

				console.Stdout(ctx).Write(b.Bytes())

				return nil
			case "table":
				return printRepositoriesTable(ctx, resp.Repositories)
			default:
				return fnerrors.BadInputError("invalid output format: %s", *output)
			}
		} else {
			req := &registryv1beta.ListImagesRequest{}
			if *matchRepo != "" {
				req.MatchRepository = &stdlib.StringMatcher{
					Values: []string{*matchRepo},
				}
			}

			resp, err := client.ContainerRegistry.ListImages(ctx, req)
			if err != nil {
				return fnerrors.InvocationError("registry", "failed to list images: %w", err)
			}

			switch *output {
			case "json":
				var b bytes.Buffer
				fmt.Fprint(&b, "[")
				for k, img := range resp.Images {
					if k > 0 {
						fmt.Fprint(&b, ",")
					}

					bb, err := protojson.MarshalOptions{UseProtoNames: true, Multiline: true}.Marshal(img)
					if err != nil {
						return err
					}

					fmt.Fprintf(&b, "\n%s", bb)
				}
				fmt.Fprint(&b, "\n]\n")

				console.Stdout(ctx).Write(b.Bytes())

				return nil
			case "table":
				return printImagesTable(ctx, resp.Images)
			default:
				return fnerrors.BadInputError("invalid output format: %s", *output)
			}
		}
	})

	return cmd
}

func newRegistryDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Get detailed information about a specific image.",
		Args:  cobra.NoArgs,
	}

	repository := cmd.Flags().String("repository", "", "Repository name (required)")
	reference := cmd.Flags().String("reference", "", "Image reference (tag or digest) (required)")
	output := cmd.Flags().StringP("output", "o", "table", "Output format: table, json")

	cmd.MarkFlagRequired("repository")
	cmd.MarkFlagRequired("reference")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		tokenSource, err := auth.LoadDefaults()
		if err != nil {
			return fnerrors.InvocationError("registry", "failed to get authentication token: %w", err)
		}

		client, err := registryapi.NewClient(ctx, tokenSource)
		if err != nil {
			return fnerrors.InvocationError("registry", "failed to create registry client: %w", err)
		}
		defer client.Close()

		req := &registryv1beta.GetImageRequest{
			Repository: *repository,
			Reference:  *reference,
		}

		resp, err := client.ContainerRegistry.GetImage(ctx, req)
		if err != nil {
			return fnerrors.InvocationError("registry", "failed to get image: %w", err)
		}

		switch *output {
		case "json":
			bb, err := protojson.MarshalOptions{UseProtoNames: true, Multiline: true}.Marshal(resp)
			if err != nil {
				return err
			}

			fmt.Fprintln(console.Stdout(ctx), string(bb))
			return nil

		case "table":
			return printImageDetails(ctx, resp)

		default:
			return fnerrors.BadInputError("invalid output format: %s", *output)
		}
	})

	return cmd
}

func newRegistryShareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "share",
		Short: "Share an image publicly or with partner accounts.",
		Args:  cobra.NoArgs,
	}

	repository := cmd.Flags().String("repository", "", "Repository name (required)")
	digest := cmd.Flags().String("digest", "", "Image digest (required)")
	visibility := cmd.Flags().String("visibility", "public", "Visibility level: public, partner")
	expiresAt := cmd.Flags().String("expires_at", "", "Expiration time in RFC3339 format (e.g., 2024-12-31T23:59:59Z)")
	output := cmd.Flags().StringP("output", "o", "table", "Output format: table, json")

	cmd.MarkFlagRequired("repository")
	cmd.MarkFlagRequired("digest")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		tokenSource, err := auth.LoadDefaults()
		if err != nil {
			return fnerrors.InvocationError("registry", "failed to get authentication token: %w", err)
		}

		client, err := registryapi.NewClient(ctx, tokenSource)
		if err != nil {
			return fnerrors.InvocationError("registry", "failed to create registry client: %w", err)
		}
		defer client.Close()

		req := &registryv1beta.ShareImageRequest{
			Repository: *repository,
			Digest:     *digest,
		}

		// Parse visibility
		switch strings.ToLower(*visibility) {
		case "public":
			req.Visibility = registryv1beta.Visibility_PUBLIC
		case "partner":
			req.Visibility = registryv1beta.Visibility_PARTNER
		default:
			return fnerrors.BadInputError("invalid visibility: %s (must be 'public' or 'partner')", *visibility)
		}

		// Parse expiration time if provided
		if *expiresAt != "" {
			t, err := time.Parse(time.RFC3339, *expiresAt)
			if err != nil {
				return fnerrors.BadInputError("invalid expires_at time format: %w", err)
			}
			req.ExpiresAt = timestamppb.New(t)
		}

		sharedImage, err := client.ContainerRegistry.ShareImage(ctx, req)
		if err != nil {
			return fnerrors.InvocationError("registry", "failed to share image: %w", err)
		}

		switch *output {
		case "json":
			bb, err := protojson.MarshalOptions{UseProtoNames: true, Multiline: true}.Marshal(sharedImage)
			if err != nil {
				return err
			}

			fmt.Fprintln(console.Stdout(ctx), string(bb))
			return nil

		case "table":
			return printSharedImage(ctx, sharedImage)

		default:
			return fnerrors.BadInputError("invalid output format: %s", *output)
		}
	})

	return cmd
}

func printRepositoriesTable(ctx context.Context, repositories []*registryv1beta.Repository) error {
	if len(repositories) == 0 {
		fmt.Fprintf(console.Stdout(ctx), "No repositories found.\n")
		return nil
	}

	cols := []tui.Column{
		{Key: "name", Title: "Repository Name", MinWidth: 20, MaxWidth: 60},
		{Key: "last_push", Title: "Last Push", MinWidth: 20, MaxWidth: 30},
	}

	rows := []tui.Row{}
	for _, repo := range repositories {
		lastPush := ""
		if repo.LastPush != nil {
			lastPush = repo.LastPush.AsTime().Format(time.RFC3339)
		}

		row := tui.Row{
			"name":      repo.Name,
			"last_push": lastPush,
		}
		rows = append(rows, row)
	}

	return tui.StaticTable(ctx, cols, rows)
}

func printImagesTable(ctx context.Context, images []*registryv1beta.Image) error {
	if len(images) == 0 {
		fmt.Fprintf(console.Stdout(ctx), "No images found.\n")
		return nil
	}

	cols := []tui.Column{
		{Key: "repository", Title: "Repository", MinWidth: 20, MaxWidth: 40},
		{Key: "digest", Title: "Digest", MinWidth: 15, MaxWidth: 30},
		{Key: "size", Title: "Size", MinWidth: 10, MaxWidth: 15},
		{Key: "created", Title: "Created", MinWidth: 20, MaxWidth: 30},
	}

	rows := []tui.Row{}
	for _, img := range images {
		created := ""
		if img.CreatedAt != nil {
			created = img.CreatedAt.AsTime().Format(time.RFC3339)
		}

		digest := img.Digest
		if len(digest) > 20 {
			digest = digest[:20] + "..."
		}

		size := "-"
		if img.Sizes != nil && img.Sizes.Total > 0 {
			size = fmt.Sprintf("%d MB", img.Sizes.Total/(1024*1024))
		}

		row := tui.Row{
			"repository": img.Repository,
			"digest":     digest,
			"size":       size,
			"created":    created,
		}
		rows = append(rows, row)
	}

	return tui.StaticTable(ctx, cols, rows)
}

func printImageDetails(ctx context.Context, resp *registryv1beta.GetImageResponse) error {
	stdout := console.Stdout(ctx)

	if resp.Image == nil {
		fmt.Fprintf(stdout, "No image found.\n")
		return nil
	}

	img := resp.Image

	fmt.Fprintf(stdout, "Repository:  %s\n", img.Repository)
	fmt.Fprintf(stdout, "Digest:      %s\n", img.Digest)

	if img.Sizes != nil && img.Sizes.Total > 0 {
		fmt.Fprintf(stdout, "Size:        %d bytes (%d MB)\n", img.Sizes.Total, img.Sizes.Total/(1024*1024))
		if len(img.Sizes.PerPlatform) > 0 {
			fmt.Fprintf(stdout, "Sizes per platform:\n")
			for platform, size := range img.Sizes.PerPlatform {
				fmt.Fprintf(stdout, "  %s: %d bytes (%d MB)\n", platform, size, size/(1024*1024))
			}
		}
	}

	if img.CreatedAt != nil {
		fmt.Fprintf(stdout, "Created At:  %s\n", img.CreatedAt.AsTime().Format(time.RFC3339))
	}

	if img.ExpiresAt != nil {
		fmt.Fprintf(stdout, "Expires At:  %s\n", img.ExpiresAt.AsTime().Format(time.RFC3339))
	}

	if len(img.Labels) > 0 {
		fmt.Fprintf(stdout, "Labels:\n")
		for _, label := range img.Labels {
			fmt.Fprintf(stdout, "  %s: %s\n", label.Name, label.Value)
		}
	}

	return nil
}

func printSharedImage(ctx context.Context, sharedImage *registryv1beta.SharedImage) error {
	stdout := console.Stdout(ctx)

	fmt.Fprintf(stdout, "Shared Image ID:  %s\n", sharedImage.Id)
	fmt.Fprintf(stdout, "Repository:       %s\n", sharedImage.SharedRepository)
	fmt.Fprintf(stdout, "Digest:           %s\n", sharedImage.SharedDigest)
	fmt.Fprintf(stdout, "Visibility:       %s\n", sharedImage.Visibility.String())

	if sharedImage.ExpiresAt != nil {
		fmt.Fprintf(stdout, "Expires At:       %s\n", sharedImage.ExpiresAt.AsTime().Format(time.RFC3339))
	} else {
		fmt.Fprintf(stdout, "Expires At:       Never\n")
	}

	if sharedImage.ImageRef != "" {
		fmt.Fprintf(stdout, "\nShared Reference: %s\n", sharedImage.ImageRef)
	}

	return nil
}
