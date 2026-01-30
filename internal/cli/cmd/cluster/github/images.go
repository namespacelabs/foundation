// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	v1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/github/v1beta"
	images "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/private/images"
	"buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/stdlib"
	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func NewImagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "base-image",
		Short: "Manage GitHub runner base images.",
	}

	cmd.AddCommand(newImagesListCmd())
	cmd.AddCommand(newImagesDescribeCmd())

	return cmd
}

func newImagesListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available GitHub runner base images.",
		Args:  cobra.NoArgs,
	}

	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")
	includeCanary := cmd.Flags().Bool("include-canary", false, "Include canary images in the output.")
	labels := cmd.Flags().StringArray("label", nil, "Filter by label (e.g., 'ubuntu-24.04'). Can be specified multiple times.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		baseImages, err := listBaseImages(ctx, *includeCanary, *labels)
		if err != nil {
			return err
		}

		stdout := console.Stdout(ctx)

		if *output == "json" {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(transformImages(baseImages)); err != nil {
				return fnerrors.InternalError("failed to encode images as JSON output: %w", err)
			}
			return nil
		}

		if len(baseImages) == 0 {
			fmt.Fprintf(stdout, "No images found.\n")
			return nil
		}

		cols := []tui.Column{
			{Key: "image", Title: "Image", MinWidth: 25, MaxWidth: 50},
			{Key: "label", Title: "Label", MinWidth: 15, MaxWidth: 25},
			{Key: "os", Title: "OS", MinWidth: 20, MaxWidth: 30},
		}

		rows := []tui.Row{}
		for _, image := range baseImages {
			imageDisplay := formatImageDisplay(image)

			osInfo := ""
			if image.Spec != nil {
				osInfo = formatOsInfo(image.Spec)
			}

			rows = append(rows, tui.Row{
				"image": imageDisplay,
				"label": image.Label,
				"os":    osInfo,
			})
		}

		return tui.StaticTable(ctx, cols, rows)
	})

	return cmd
}

func newImagesDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe <label>",
		Short: "Show details for a specific GitHub runner base image.",
		Args:  cobra.ExactArgs(1),
	}

	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")
	canary := cmd.Flags().Bool("canary", false, "Fetch the canary/staging image instead of production.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		label := args[0]

		stageName := "production"
		releaseStages := []images.ReleaseStage{images.ReleaseStage_PRODUCTION}
		if *canary {
			stageName = "canary"
			releaseStages = []images.ReleaseStage{images.ReleaseStage_CANARY}
		}

		baseImages, err := listBaseImagesWithStages(ctx, []string{label}, releaseStages)
		if err != nil {
			return err
		}

		if len(baseImages) == 0 {
			return fnerrors.Newf("no %s image found with label: %s", stageName, label)
		}

		if len(baseImages) > 1 {
			return fnerrors.Newf("multiple %s images found with label %s (expected 1, got %d)", stageName, label, len(baseImages))
		}

		image := baseImages[0]
		stdout := console.Stdout(ctx)

		if *output == "json" {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(transformImageForOutput(image)); err != nil {
				return fnerrors.InternalError("failed to encode image as JSON output: %w", err)
			}
			return nil
		}

		printImageDetails(stdout, image)
		return nil
	})

	return cmd
}

func printImageDetails(stdout io.Writer, image *v1beta.RunnerBaseImage) {
	imageDisplay := formatImageDisplay(image)

	fmt.Fprintf(stdout, "\n%s\n\n", imageDisplay)
	fmt.Fprintf(stdout, "Label:       %s\n", image.Label)
	fmt.Fprintf(stdout, "Image Ref:   %s\n", image.ImageRef)

	if spec := image.Spec; spec != nil {
		if osInfo := formatOsInfo(spec); osInfo != "" {
			fmt.Fprintf(stdout, "OS:          %s\n", osInfo)
		}
		if spec.Id != "" {
			fmt.Fprintf(stdout, "ID:          %s\n", spec.Id)
		}
		if spec.UpdatedAt != nil {
			fmt.Fprintf(stdout, "Updated At:  %s\n", spec.UpdatedAt.AsTime().Format("2006-01-02 15:04:05 UTC"))
		}

		if len(spec.Features) > 0 {
			fmt.Fprintf(stdout, "\nFeatures:\n")
			for _, f := range spec.Features {
				fmt.Fprintf(stdout, "  - %s\n", f)
			}
		}

		if len(spec.DefaultPackages) > 0 {
			fmt.Fprintf(stdout, "\nDefault Packages:\n")
			for _, pkg := range spec.DefaultPackages {
				printPackage(stdout, pkg)
			}
		}

		if len(spec.Packages) > 0 {
			fmt.Fprintf(stdout, "\nInstalled Packages:\n")
			for _, pkg := range spec.Packages {
				printPackage(stdout, pkg)
			}
		}
	}
	fmt.Fprintln(stdout)
}

func printPackage(stdout io.Writer, pkg *images.Package) {
	line := fmt.Sprintf("  - %s", pkg.Name)
	if pkg.Version != "" {
		line += fmt.Sprintf(" %s", pkg.Version)
	}
	if pkg.Build != "" {
		line += fmt.Sprintf(" (build %s)", pkg.Build)
	}
	if pkg.Type != "" {
		line += fmt.Sprintf(" [%s]", pkg.Type)
	}
	if pkg.IsDefault {
		line += " (default)"
	}
	fmt.Fprintln(stdout, line)
}

func listBaseImages(ctx context.Context, includeStaging bool, labels []string) ([]*v1beta.RunnerBaseImage, error) {
	var stages []images.ReleaseStage
	if includeStaging {
		stages = []images.ReleaseStage{
			images.ReleaseStage_PRODUCTION,
			images.ReleaseStage_CANARY,
		}
	}
	return listBaseImagesWithStages(ctx, labels, stages)
}

func listBaseImagesWithStages(ctx context.Context, labels []string, stages []images.ReleaseStage) ([]*v1beta.RunnerBaseImage, error) {
	client, err := fnapi.NewImageServiceClient(ctx)
	if err != nil {
		return nil, err
	}

	listReq := &v1beta.ListBaseImagesRequest{}
	if len(stages) > 0 {
		listReq.ReleaseStages = stages
	}
	if len(labels) > 0 {
		listReq.Labels = &stdlib.StringMatcher{
			Op:     stdlib.StringMatcher_IS_ANY_OF,
			Values: labels,
		}
	}

	req := connect.NewRequest(listReq)

	res, err := client.ListBaseImages(ctx, req)
	if err != nil {
		return nil, err
	}

	return res.Msg.Images, nil
}

func transformImages(images []*v1beta.RunnerBaseImage) []map[string]any {
	var result []map[string]any
	for _, image := range images {
		result = append(result, transformImageForOutput(image))
	}
	return result
}

func transformImageForOutput(image *v1beta.RunnerBaseImage) map[string]any {
	m := map[string]any{
		"label":     image.Label,
		"image_ref": image.ImageRef,
	}

	if image.Title != "" {
		m["title"] = image.Title
	}

	if spec := image.Spec; spec != nil {
		m["id"] = spec.Id
		m["description"] = spec.Description
		m["os"] = spec.Os
		m["os_name"] = spec.OsName
		m["os_version"] = spec.OsVersion

		if spec.ReleaseStage != images.ReleaseStage_STAGE_UNKNOWN {
			m["release_stage"] = formatReleaseStage(spec.ReleaseStage)
		}

		if spec.UpdatedAt != nil {
			m["updated_at"] = spec.UpdatedAt.AsTime()
		}

		if len(spec.Features) > 0 {
			m["features"] = spec.Features
		}

		if len(spec.Packages) > 0 {
			m["packages"] = transformPackages(spec.Packages)
		}

		if len(spec.DefaultPackages) > 0 {
			m["default_packages"] = transformPackages(spec.DefaultPackages)
		}
	}

	return m
}

func transformPackages(packages []*images.Package) []map[string]any {
	var result []map[string]any
	for _, pkg := range packages {
		p := map[string]any{
			"name": pkg.Name,
		}
		if pkg.Type != "" {
			p["type"] = pkg.Type
		}
		if pkg.Version != "" {
			p["version"] = pkg.Version
		}
		if pkg.Build != "" {
			p["build"] = pkg.Build
		}
		if pkg.IsDefault {
			p["is_default"] = pkg.IsDefault
		}
		result = append(result, p)
	}
	return result
}

func formatImageDisplay(image *v1beta.RunnerBaseImage) string {
	spec := image.Spec

	var display string
	if image.Title != "" {
		osPrefix := ""
		if spec != nil {
			osPrefix = formatOsDisplayName(spec.Os)
		}
		if osPrefix != "" {
			display = fmt.Sprintf("%s %s", osPrefix, image.Title)
		} else {
			display = image.Title
		}
	} else if spec != nil {
		display = spec.Description
	}

	if spec != nil {
		if stage := formatReleaseStage(spec.ReleaseStage); stage != "" && stage != "production" {
			display = fmt.Sprintf("%s (%s)", display, stage)
		}
	}

	return display
}

func formatOsDisplayName(os string) string {
	switch os {
	case "macos":
		return "MacOS"
	case "linux":
		return "Linux"
	case "windows":
		return "Windows"
	default:
		return os
	}
}

func formatOsInfo(spec *images.BaseImage) string {
	if spec.OsName != "" && spec.OsVersion != "" {
		return fmt.Sprintf("%s %s", spec.OsName, spec.OsVersion)
	}
	if spec.OsName != "" {
		return spec.OsName
	}
	return spec.Os
}

func formatReleaseStage(stage images.ReleaseStage) string {
	switch stage {
	case images.ReleaseStage_PRODUCTION:
		return "production"
	case images.ReleaseStage_CANARY:
		return "canary"
	default:
		return ""
	}
}
