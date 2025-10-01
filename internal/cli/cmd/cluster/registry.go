// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"syscall"
	"time"

	registryv1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/registry/v1beta"
	"buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/stdlib"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/integrations/api/builds"
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

// parseImageReference parses an image reference in the format "repository:tag" or "repository@digest"
// and returns the repository and reference parts. If nscrBase is provided and matches, it is stripped.
func parseImageReference(imageRef, nscrBase string) (repository, reference string, err error) {
	// Strip nscr base if it matches
	if nscrBase != "" && strings.HasPrefix(imageRef, nscrBase+"/") {
		imageRef = strings.TrimPrefix(imageRef, nscrBase+"/")
	}

	// Check for digest format (repository@sha256:...)
	if idx := strings.Index(imageRef, "@"); idx != -1 {
		return imageRef[:idx], imageRef[idx+1:], nil
	}

	// Check for tag format (repository:tag)
	// Use LastIndex to handle registry URLs like registry.example.com:5000/path:tag
	if idx := strings.LastIndex(imageRef, ":"); idx != -1 {
		return imageRef[:idx], imageRef[idx+1:], nil
	}

	return "", "", fmt.Errorf("invalid format, expected 'repository:tag' or 'repository@digest'")
}

// formatImageReference formats a complete image reference with nscr base prefix.
func formatImageReference(nscrBase, repository, digest string) string {
	if nscrBase != "" {
		return fmt.Sprintf("%s/%s@%s", nscrBase, repository, digest)
	}
	return fmt.Sprintf("%s@%s", repository, digest)
}

// formatImageReferenceStyled formats an image reference with styling if terminal supports it.
// The nscr base prefix is dimmed to emphasize the repository and digest.
func formatImageReferenceStyled(nscrBase, repository, digest string) string {
	ref := formatImageReference(nscrBase, repository, digest)

	// Only apply styling if stdout is a terminal
	if !term.IsTerminal(int(syscall.Stdout)) {
		return ref
	}

	// If no nscr base, return unstyled
	if nscrBase == "" {
		return ref
	}

	// Use adaptive color that works in both light and dark modes
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#999999", // Medium gray for light mode
		Dark:  "#666666", // Darker gray for dark mode
	})

	// Split and style: dimmed base + normal rest
	styledBase := dimStyle.Render(nscrBase + "/")
	return styledBase + fmt.Sprintf("%s@%s", repository, digest)
}

// resolveImageReference resolves repository and reference from positional args and flags.
// Positional arg takes precedence unless overridden by explicit flags.
func resolveImageReference(args []string, repositoryFlag, referenceFlag, nscrBase string) (repository, reference string, err error) {
	// Parse positional argument if provided
	if len(args) > 0 {
		repository, reference, err = parseImageReference(args[0], nscrBase)
		if err != nil {
			return "", "", err
		}
	}

	// Flags override positional argument
	if repositoryFlag != "" {
		repository = repositoryFlag
	}
	if referenceFlag != "" {
		reference = referenceFlag
	}

	// Validate required fields
	if repository == "" {
		return "", "", fmt.Errorf("repository is required (provide image reference as argument or use --repository)")
	}
	if reference == "" {
		return "", "", fmt.Errorf("reference is required (provide image reference as argument or use --reference)")
	}

	return repository, reference, nil
}

func newRegistryListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [repository]",
		Short: "List images or repositories in the registry.",
		Args:  cobra.MaximumNArgs(1),
	}

	repositories := cmd.Flags().Bool("repositories", false, "List repositories instead of images")
	output := cmd.Flags().StringP("output", "o", "table", "Output format: table, json")
	matchRepo := cmd.Flags().String("repository", "", "Filter images by repository name")
	includeDeleted := cmd.Flags().Bool("include_deleted", false, "Include deleted images in results")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		// Use positional argument if provided, unless --repository flag is explicitly set
		repository := *matchRepo
		if len(args) > 0 && repository == "" {
			repository = args[0]
		}
		tokenSource, err := auth.LoadDefaults()
		if err != nil {
			return fnerrors.InvocationError("registry", "failed to get authentication token: %w", err)
		}

		nscrBase, err := builds.NSCRBase(ctx, tokenSource)
		if err != nil {
			return fnerrors.InvocationError("registry", "failed to get registry base: %w", err)
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
			if repository != "" {
				req.MatchRepository = &stdlib.StringMatcher{
					Values: []string{repository},
					Op:     stdlib.StringMatcher_IS_ANY_OF,
				}
			}

			resp, err := client.ContainerRegistry.ListImages(ctx, req)
			if err != nil {
				return fnerrors.InvocationError("registry", "failed to list images: %w", err)
			}

			// Filter out deleted images unless include_deleted is set
			images := resp.Images
			if !*includeDeleted {
				var filtered []*registryv1beta.Image
				for _, img := range resp.Images {
					if img.DeletedAt == nil {
						filtered = append(filtered, img)
					}
				}
				images = filtered
			}

			switch *output {
			case "json":
				// Convert to JSON and add image_ref field
				var result []map[string]interface{}
				for _, img := range images {
					// Marshal to JSON first
					bb, err := protojson.Marshal(img)
					if err != nil {
						return err
					}

					var imgMap map[string]interface{}
					if err := json.Unmarshal(bb, &imgMap); err != nil {
						return err
					}

					// Add image_ref field at the beginning
					imageRef := formatImageReference(nscrBase, img.Repository, img.Digest)
					imgMap["image_ref"] = imageRef

					result = append(result, imgMap)
				}

				// Marshal back to JSON with indentation
				bb, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return err
				}

				fmt.Fprintln(console.Stdout(ctx), string(bb))
				return nil
			case "table":
				return printImagesTable(ctx, nscrBase, images)
			default:
				return fnerrors.BadInputError("invalid output format: %s", *output)
			}
		}
	})

	return cmd
}

func newRegistryDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe [image-reference]",
		Short: "Get detailed information about a specific image.",
		Args:  cobra.MaximumNArgs(1),
		Long: `Get detailed information about a specific image.

The image reference can be provided as a positional argument in the format:
  - repository:tag (e.g., myrepo:latest)
  - repository@digest (e.g., myrepo@sha256:abc123...)

Alternatively, use --repository and --reference flags.`,
	}

	repositoryFlag := cmd.Flags().String("repository", "", "Repository name")
	referenceFlag := cmd.Flags().String("reference", "", "Image reference (tag or digest)")
	output := cmd.Flags().StringP("output", "o", "table", "Output format: table, json")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		tokenSource, err := auth.LoadDefaults()
		if err != nil {
			return fnerrors.InvocationError("registry", "failed to get authentication token: %w", err)
		}

		nscrBase, err := builds.NSCRBase(ctx, tokenSource)
		if err != nil {
			return fnerrors.InvocationError("registry", "failed to get registry base: %w", err)
		}

		repository, reference, err := resolveImageReference(args, *repositoryFlag, *referenceFlag, nscrBase)
		if err != nil {
			return fnerrors.BadInputError("%w", err)
		}

		client, err := registryapi.NewClient(ctx, tokenSource)
		if err != nil {
			return fnerrors.InvocationError("registry", "failed to create registry client: %w", err)
		}
		defer client.Close()

		req := &registryv1beta.GetImageRequest{
			Repository: repository,
			Reference:  reference,
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
			return printImageDetails(ctx, nscrBase, resp)

		default:
			return fnerrors.BadInputError("invalid output format: %s", *output)
		}
	})

	return cmd
}

func newRegistryShareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "share [image-reference]",
		Short: "Share an image publicly or with partner accounts.",
		Args:  cobra.MaximumNArgs(1),
		Long: `Share an image publicly or with partner accounts.

The image reference can be provided as a positional argument in the format:
  - repository:tag (e.g., myrepo:latest)
  - repository@digest (e.g., myrepo@sha256:abc123...)

Alternatively, use --repository and --digest flags.`,
	}

	repositoryFlag := cmd.Flags().String("repository", "", "Repository name")
	digestFlag := cmd.Flags().String("digest", "", "Image digest")
	visibility := cmd.Flags().String("visibility", "public", "Visibility level: public, partner")
	expiresAt := cmd.Flags().String("expires_at", "", "Expiration time in RFC3339 format (e.g., 2024-12-31T23:59:59Z)")
	output := cmd.Flags().StringP("output", "o", "table", "Output format: table, json")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		tokenSource, err := auth.LoadDefaults()
		if err != nil {
			return fnerrors.InvocationError("registry", "failed to get authentication token: %w", err)
		}

		nscrBase, err := builds.NSCRBase(ctx, tokenSource)
		if err != nil {
			return fnerrors.InvocationError("registry", "failed to get registry base: %w", err)
		}

		repository, digest, err := resolveImageReference(args, *repositoryFlag, *digestFlag, nscrBase)
		if err != nil {
			return fnerrors.BadInputError("%w", err)
		}

		client, err := registryapi.NewClient(ctx, tokenSource)
		if err != nil {
			return fnerrors.InvocationError("registry", "failed to create registry client: %w", err)
		}
		defer client.Close()

		req := &registryv1beta.ShareImageRequest{
			Repository: repository,
			Digest:     digest,
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

func printImagesTable(ctx context.Context, nscrBase string, images []*registryv1beta.Image) error {
	if len(images) == 0 {
		fmt.Fprintf(console.Stdout(ctx), "No images found.\n")
		return nil
	}

	cols := []tui.Column{
		{Key: "reference", Title: "Image Reference", MinWidth: 30, MaxWidth: 80},
		{Key: "size", Title: "Size", MinWidth: 10, MaxWidth: 15},
		{Key: "created", Title: "Created", MinWidth: 20, MaxWidth: 30},
	}

	rows := []tui.Row{}
	for _, img := range images {
		created := ""
		if img.CreatedAt != nil {
			created = img.CreatedAt.AsTime().Format(time.RFC3339)
		}

		// Build complete image reference with styling
		imageRef := formatImageReferenceStyled(nscrBase, img.Repository, img.Digest)

		// Check length of unstyled version for truncation
		unstyledRef := formatImageReference(nscrBase, img.Repository, img.Digest)
		if len(unstyledRef) > 77 {
			// Truncate the unstyled version and re-apply styling
			truncated := unstyledRef[:77] + "..."
			imageRef = truncated // Fall back to unstyled if truncated
		}

		size := "-"
		if img.Sizes != nil && img.Sizes.Total > 0 {
			size = humanize.IBytes(uint64(img.Sizes.Total))
		}
		// Right-align size within 10 characters
		size = fmt.Sprintf("%10s", size)

		row := tui.Row{
			"reference": imageRef,
			"size":      size,
			"created":   created,
		}
		rows = append(rows, row)
	}

	return tui.StaticTable(ctx, cols, rows)
}

func printImageDetails(ctx context.Context, nscrBase string, resp *registryv1beta.GetImageResponse) error {
	stdout := console.Stdout(ctx)

	if resp.Image == nil {
		fmt.Fprintf(stdout, "No image found.\n")
		return nil
	}

	img := resp.Image

	// Show complete image reference first with styling
	imageRef := formatImageReferenceStyled(nscrBase, img.Repository, img.Digest)
	fmt.Fprintf(stdout, "Image Reference: %s\n", imageRef)
	fmt.Fprintf(stdout, "Repository:      %s\n", img.Repository)
	fmt.Fprintf(stdout, "Digest:          %s\n", img.Digest)

	if img.Sizes != nil && img.Sizes.Total > 0 {
		fmt.Fprintf(stdout, "Size:            %s\n", humanize.IBytes(uint64(img.Sizes.Total)))
		if len(img.Sizes.PerPlatform) > 0 {
			fmt.Fprintf(stdout, "Sizes per platform:\n")
			for platform, size := range img.Sizes.PerPlatform {
				fmt.Fprintf(stdout, "  %s: %s\n", platform, humanize.IBytes(uint64(size)))
			}
		}
	}

	if img.CreatedAt != nil {
		fmt.Fprintf(stdout, "Created At:      %s\n", img.CreatedAt.AsTime().Format(time.RFC3339))
	}

	if img.ExpiresAt != nil {
		fmt.Fprintf(stdout, "Expires At:      %s\n", img.ExpiresAt.AsTime().Format(time.RFC3339))
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
