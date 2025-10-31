// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package github

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	computev1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/compute/v1beta"
	v1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/github/v1beta"
	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/files"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func NewProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage GitHub runner profiles.",
	}

	cmd.AddCommand(newProfileCreateCmd())
	cmd.AddCommand(newProfileListCmd())
	cmd.AddCommand(newProfileDescribeCmd())
	cmd.AddCommand(newProfileUpdateCmd())
	cmd.AddCommand(newProfileDeleteCmd())

	return cmd
}

func newProfileCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new GitHub runner profile.",
		Args:  cobra.NoArgs,
	}

	specFile := cmd.Flags().String("spec_file", "", "Path to JSON file containing the profile spec. When provided, individual flags are ignored.")
	tag := cmd.Flags().String("tag", "", "Stable user-configurable alias for the profile (required unless --spec_file is used).")
	description := cmd.Flags().String("description", "", "Human-friendly description of the profile.")
	os := cmd.Flags().String("os", "ubuntu-24.10", "Operating system label (e.g., 'ubuntu-24.10').")
	vcpu := cmd.Flags().Int32("vcpu", 4, "Number of virtual CPUs.")
	memoryMB := cmd.Flags().Int32("memory_mb", 16384, "Memory in megabytes.")
	machineArch := cmd.Flags().String("machine_arch", "amd64", "Machine architecture (amd64 or arm64).")
	builderMode := cmd.Flags().String("builder_mode", "BUILDER_MODE_AUTO", "Builder mode (BUILDER_MODE_AUTO, BUILDER_MODE_DISABLED, BUILDER_MODE_FORCED).")
	emoji := cmd.Flags().String("emoji", "", "Optional emoji to visually identify the profile.")
	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		var spec *v1beta.RunnerProfileSpec

		// If spec file is provided, read the entire spec from the file
		if *specFile != "" {
			spec = &v1beta.RunnerProfileSpec{}
			if err := files.ReadJson(*specFile, spec); err != nil {
				return fnerrors.Newf("failed to read spec file: %w", err)
			}
		} else {
			// Otherwise, build spec from flags
			if *tag == "" {
				return fnerrors.New("--tag is required")
			}

			// Parse builder mode
			builderModeValue, ok := v1beta.BuilderMode_value[*builderMode]
			if !ok {
				return fnerrors.Newf("invalid builder mode: %s", *builderMode)
			}

			spec = &v1beta.RunnerProfileSpec{
				Tag:         *tag,
				Description: *description,
				Os:          *os,
				InstanceShape: &computev1beta.InstanceShape{
					VirtualCpu:      *vcpu,
					MemoryMegabytes: *memoryMB,
					MachineArch:     *machineArch,
					Os:              "linux",
				},
				BuilderMode: v1beta.BuilderMode(builderModeValue),
				Emoji:       *emoji,
			}
		}

		profile, err := createProfile(ctx, spec)
		if err != nil {
			return err
		}

		stdout := console.Stdout(ctx)

		if *output == "json" {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(transformProfileForOutput(profile)); err != nil {
				return fnerrors.InternalError("failed to encode profile as JSON output: %w", err)
			}
			return nil
		}

		fmt.Fprintf(stdout, "\nProfile created successfully:\n")
		fmt.Fprintf(stdout, "  Profile ID: %s\n", profile.ProfileId)
		fmt.Fprintf(stdout, "  Tag: %s\n", profile.Spec.Tag)
		fmt.Fprintf(stdout, "  Description: %s\n", profile.Spec.Description)
		fmt.Fprintf(stdout, "  OS: %s\n", profile.Spec.Os)
		fmt.Fprintf(stdout, "  Version: %d\n", profile.Version)

		return nil
	})

	return cmd
}

func newProfileListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all GitHub runner profiles.",
		Args:  cobra.NoArgs,
	}

	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		profiles, err := listProfiles(ctx)
		if err != nil {
			return err
		}

		stdout := console.Stdout(ctx)

		if *output == "json" {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(transformProfiles(profiles)); err != nil {
				return fnerrors.InternalError("failed to encode profiles as JSON output: %w", err)
			}
			return nil
		}

		if len(profiles) == 0 {
			fmt.Fprintf(stdout, "No profiles found.\n")
			return nil
		}

		cols := []tui.Column{
			{Key: "profile_id", Title: "ID", MinWidth: 10, MaxWidth: 30},
			{Key: "tag", Title: "Tag", MinWidth: 10, MaxWidth: 30},
			{Key: "os", Title: "OS", MinWidth: 10, MaxWidth: 20},
			{Key: "shape", Title: "Shape", MinWidth: 8, MaxWidth: 15},
			{Key: "platform", Title: "Platform", MinWidth: 12, MaxWidth: 20},
		}

		rows := []tui.Row{}
		for _, profile := range profiles {
			shape := "-"
			platform := "-"
			if profile.Spec.InstanceShape != nil {
				// Format shape as "CPUxMemory" e.g. "4x16"
				memoryGB := profile.Spec.InstanceShape.MemoryMegabytes / 1024
				shape = fmt.Sprintf("%dx%d", profile.Spec.InstanceShape.VirtualCpu, memoryGB)

				// Format platform as "os/arch" e.g. "linux/amd64"
				osType := profile.Spec.InstanceShape.Os
				if osType == "" {
					osType = "linux"
				}
				platform = fmt.Sprintf("%s/%s", osType, profile.Spec.InstanceShape.MachineArch)
			}

			rows = append(rows, tui.Row{
				"profile_id": profile.ProfileId,
				"tag":        profile.Spec.Tag,
				"os":         profile.Spec.Os,
				"shape":      shape,
				"platform":   platform,
			})
		}

		return tui.StaticTable(ctx, cols, rows)
	})

	return cmd
}

func newProfileDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Describe a GitHub runner profile in detail.",
		Args:  cobra.NoArgs,
	}

	profileId := cmd.Flags().String("profile_id", "", "Profile ID to describe (required).")
	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *profileId == "" {
			return fnerrors.New("--profile_id is required")
		}

		profiles, err := listProfiles(ctx)
		if err != nil {
			return err
		}

		var profile *v1beta.RunnerProfileWithStatus
		for _, p := range profiles {
			if p.ProfileId == *profileId {
				profile = p
				break
			}
		}

		if profile == nil {
			return fnerrors.Newf("profile not found: %s", *profileId)
		}

		stdout := console.Stdout(ctx)

		if *output == "json" {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(transformProfileForOutput(profile)); err != nil {
				return fnerrors.InternalError("failed to encode profile as JSON output: %w", err)
			}
			return nil
		}

		// Plain text detailed output
		fmt.Fprintf(stdout, "\nProfile Details:\n\n")
		fmt.Fprintf(stdout, "Profile ID:   %s\n", profile.ProfileId)
		fmt.Fprintf(stdout, "Tag:          %s\n", profile.Spec.Tag)
		if profile.Spec.Emoji != "" {
			fmt.Fprintf(stdout, "Emoji:        %s\n", profile.Spec.Emoji)
		}
		if profile.Spec.Description != "" {
			fmt.Fprintf(stdout, "Description:  %s\n", profile.Spec.Description)
		}
		fmt.Fprintf(stdout, "OS:           %s\n", profile.Spec.Os)

		if profile.Spec.InstanceShape != nil {
			fmt.Fprintf(stdout, "\nInstance Shape:\n")
			fmt.Fprintf(stdout, "  CPU:        %d vCPU\n", profile.Spec.InstanceShape.VirtualCpu)
			memoryGB := float64(profile.Spec.InstanceShape.MemoryMegabytes) / 1024
			fmt.Fprintf(stdout, "  Memory:     %.0f GB (%d MB)\n", memoryGB, profile.Spec.InstanceShape.MemoryMegabytes)
			fmt.Fprintf(stdout, "  Arch:       %s\n", profile.Spec.InstanceShape.MachineArch)
			osType := profile.Spec.InstanceShape.Os
			if osType == "" {
				osType = "linux"
			}
			fmt.Fprintf(stdout, "  Platform:   %s/%s\n", osType, profile.Spec.InstanceShape.MachineArch)
		}

		if profile.Spec.BuilderMode != v1beta.BuilderMode_BUILDER_MODE_UNSPECIFIED {
			fmt.Fprintf(stdout, "\nBuilder Mode: %s\n", profile.Spec.BuilderMode.String())
		}

		if len(profile.Spec.CacheVolumeSettings) > 0 {
			fmt.Fprintf(stdout, "\nCache Volumes: %d configured\n", len(profile.Spec.CacheVolumeSettings))
		}

		if len(profile.Spec.ExperimentalFeatures) > 0 {
			fmt.Fprintf(stdout, "\nExperimental Features:\n")
			for _, feature := range profile.Spec.ExperimentalFeatures {
				fmt.Fprintf(stdout, "  - %s\n", feature)
			}
		}

		fmt.Fprintf(stdout, "\nMetadata:\n")
		fmt.Fprintf(stdout, "  Version:    %d\n", profile.Version)
		if profile.CreatedAt != nil {
			fmt.Fprintf(stdout, "  Created:    %s\n", profile.CreatedAt.AsTime().Format(time.RFC3339))
		}
		if profile.UpdatedAt != nil {
			fmt.Fprintf(stdout, "  Updated:    %s\n", profile.UpdatedAt.AsTime().Format(time.RFC3339))
		}

		if profile.Status != nil && len(profile.Status.CustomRunnerImage) > 0 {
			fmt.Fprintf(stdout, "\nCustom Runner Images: %d\n", len(profile.Status.CustomRunnerImage))
		}

		fmt.Fprintf(stdout, "\n")
		return nil
	})

	return cmd
}

func newProfileUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an existing GitHub runner profile.",
		Args:  cobra.NoArgs,
	}

	profileId := cmd.Flags().String("profile_id", "", "Profile ID to update (required).")
	specFile := cmd.Flags().String("spec_file", "", "Path to JSON file containing the profile spec. When provided, individual flags are ignored.")
	tag := cmd.Flags().String("tag", "", "Stable user-configurable alias for the profile.")
	description := cmd.Flags().String("description", "", "Human-friendly description of the profile.")
	os := cmd.Flags().String("os", "", "Operating system label (e.g., 'ubuntu-24.10').")
	vcpu := cmd.Flags().Int32("vcpu", 0, "Number of virtual CPUs.")
	memoryMB := cmd.Flags().Int32("memory_mb", 0, "Memory in megabytes.")
	machineArch := cmd.Flags().String("machine_arch", "", "Machine architecture (amd64 or arm64).")
	builderMode := cmd.Flags().String("builder_mode", "", "Builder mode (BUILDER_MODE_AUTO, BUILDER_MODE_DISABLED, BUILDER_MODE_FORCED).")
	emoji := cmd.Flags().String("emoji", "", "Optional emoji to visually identify the profile.")
	version := cmd.Flags().Int64("version", 0, "Current version of the profile for optimistic concurrency control (required).")
	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *profileId == "" {
			return fnerrors.New("--profile_id is required")
		}
		if *version == 0 {
			return fnerrors.New("--version is required")
		}

		var spec *v1beta.RunnerProfileSpec

		// If spec file is provided, read the entire spec from the file
		if *specFile != "" {
			spec = &v1beta.RunnerProfileSpec{}
			if err := files.ReadJson(*specFile, spec); err != nil {
				return fnerrors.Newf("failed to read spec file: %w", err)
			}
		} else {
			// Otherwise, get the current profile and apply individual flag updates
			profiles, err := listProfiles(ctx)
			if err != nil {
				return err
			}

			var currentProfile *v1beta.RunnerProfileWithStatus
			for _, p := range profiles {
				if p.ProfileId == *profileId {
					currentProfile = p
					break
				}
			}

			if currentProfile == nil {
				return fnerrors.Newf("profile not found: %s", *profileId)
			}

			// Start with the current spec
			spec = currentProfile.Spec

			// Update only the fields that were provided
			if *tag != "" {
				spec.Tag = *tag
			}
			if cmd.Flags().Changed("description") {
				spec.Description = *description
			}
			if *os != "" {
				spec.Os = *os
			}
			if *emoji != "" {
				spec.Emoji = *emoji
			}
			if *builderMode != "" {
				builderModeValue, ok := v1beta.BuilderMode_value[*builderMode]
				if !ok {
					return fnerrors.Newf("invalid builder mode: %s", *builderMode)
				}
				spec.BuilderMode = v1beta.BuilderMode(builderModeValue)
			}

			// Update instance shape fields if provided
			if *vcpu != 0 || *memoryMB != 0 || *machineArch != "" {
				if spec.InstanceShape == nil {
					spec.InstanceShape = &computev1beta.InstanceShape{}
				}
				if *vcpu != 0 {
					spec.InstanceShape.VirtualCpu = *vcpu
				}
				if *memoryMB != 0 {
					spec.InstanceShape.MemoryMegabytes = *memoryMB
				}
				if *machineArch != "" {
					spec.InstanceShape.MachineArch = *machineArch
				}
			}
		}

		profile, err := updateProfile(ctx, *profileId, spec, *version)
		if err != nil {
			return err
		}

		stdout := console.Stdout(ctx)

		if *output == "json" {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(transformProfileForOutput(profile)); err != nil {
				return fnerrors.InternalError("failed to encode profile as JSON output: %w", err)
			}
			return nil
		}

		fmt.Fprintf(stdout, "\nProfile updated successfully:\n")
		fmt.Fprintf(stdout, "  Profile ID: %s\n", profile.ProfileId)
		fmt.Fprintf(stdout, "  Tag: %s\n", profile.Spec.Tag)
		fmt.Fprintf(stdout, "  Description: %s\n", profile.Spec.Description)
		fmt.Fprintf(stdout, "  Version: %d\n", profile.Version)

		return nil
	})

	return cmd
}

func newProfileDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a GitHub runner profile.",
		Args:  cobra.NoArgs,
	}

	profileId := cmd.Flags().String("profile_id", "", "Profile ID to delete (required).")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *profileId == "" {
			return fnerrors.New("--profile_id is required")
		}

		if err := deleteProfile(ctx, *profileId); err != nil {
			return err
		}

		stdout := console.Stdout(ctx)
		fmt.Fprintf(stdout, "Profile %s deleted successfully.\n", *profileId)

		return nil
	})

	return cmd
}

// CRUD helper functions

func createProfile(ctx context.Context, spec *v1beta.RunnerProfileSpec) (*v1beta.RunnerProfileWithStatus, error) {
	client, err := fnapi.NewProfileServiceClient(ctx)
	if err != nil {
		return nil, err
	}

	req := connect.NewRequest(&v1beta.CreateProfileRequest{
		Spec: spec,
	})

	res, err := client.CreateProfile(ctx, req)
	if err != nil {
		return nil, err
	}

	return res.Msg.Profile, nil
}

func listProfiles(ctx context.Context) ([]*v1beta.RunnerProfileWithStatus, error) {
	client, err := fnapi.NewProfileServiceClient(ctx)
	if err != nil {
		return nil, err
	}

	req := connect.NewRequest(&emptypb.Empty{})

	res, err := client.ListProfiles(ctx, req)
	if err != nil {
		return nil, err
	}

	return res.Msg.Profiles, nil
}

func updateProfile(ctx context.Context, profileId string, spec *v1beta.RunnerProfileSpec, updateVersion int64) (*v1beta.RunnerProfileWithStatus, error) {
	client, err := fnapi.NewProfileServiceClient(ctx)
	if err != nil {
		return nil, err
	}

	req := connect.NewRequest(&v1beta.UpdateProfileRequest{
		ProfileId:     profileId,
		Spec:          spec,
		UpdateVersion: updateVersion,
	})

	res, err := client.UpdateProfile(ctx, req)
	if err != nil {
		return nil, err
	}

	return res.Msg.Profile, nil
}

func deleteProfile(ctx context.Context, profileId string) error {
	client, err := fnapi.NewProfileServiceClient(ctx)
	if err != nil {
		return err
	}

	req := connect.NewRequest(&v1beta.DeleteProfileRequest{
		ProfileId: profileId,
	})

	_, err = client.DeleteProfile(ctx, req)
	return err
}

// Transform functions for JSON output

func transformProfiles(profiles []*v1beta.RunnerProfileWithStatus) []map[string]any {
	var result []map[string]any
	for _, profile := range profiles {
		result = append(result, transformProfileForOutput(profile))
	}
	return result
}

// transformProfileForOutput creates a validated set of output fields which should not change.
func transformProfileForOutput(profile *v1beta.RunnerProfileWithStatus) map[string]any {
	m := map[string]any{
		"profile_id":  profile.ProfileId,
		"tag":         profile.Spec.Tag,
		"description": profile.Spec.Description,
		"os":          profile.Spec.Os,
		"version":     profile.Version,
	}

	if profile.Spec.Emoji != "" {
		m["emoji"] = profile.Spec.Emoji
	}

	if profile.Spec.InstanceShape != nil {
		m["instance_shape"] = map[string]any{
			"virtual_cpu":      profile.Spec.InstanceShape.VirtualCpu,
			"memory_megabytes": profile.Spec.InstanceShape.MemoryMegabytes,
			"machine_arch":     profile.Spec.InstanceShape.MachineArch,
			"os":               profile.Spec.InstanceShape.Os,
		}
	}

	if profile.Spec.BuilderMode != v1beta.BuilderMode_BUILDER_MODE_UNSPECIFIED {
		m["builder_mode"] = profile.Spec.BuilderMode.String()
	}

	if len(profile.Spec.ExperimentalFeatures) > 0 {
		m["experimental_features"] = profile.Spec.ExperimentalFeatures
	}

	if profile.CreatedAt != nil {
		m["created_at"] = profile.CreatedAt.AsTime()
	}

	if profile.UpdatedAt != nil {
		m["updated_at"] = profile.UpdatedAt.AsTime()
	}

	return m
}
