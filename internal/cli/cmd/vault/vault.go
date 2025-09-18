// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package vault

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	v1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/vault/v1beta"
	"buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/stdlib"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/integrations/api/vault"
	"namespacelabs.dev/integrations/auth"
)

func NewVaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vault",
		Short: "Manage secrets in the vault.",
	}

	cmd.AddCommand(NewListCmd())
	cmd.AddCommand(NewAddCmd())
	cmd.AddCommand(NewSetCmd())
	cmd.AddCommand(NewDeleteCmd())

	return cmd
}

func NewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List objects in the vault.",
		Args:  cobra.NoArgs,
	}

	output := cmd.Flags().StringP("output", "o", "table", "Output format: table, json")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		tokenSource, err := auth.LoadDefaults()
		if err != nil {
			return fnerrors.InvocationError("vault", "failed to get authentication token: %w", err)
		}

		client, err := vault.NewClient(ctx, tokenSource)
		if err != nil {
			return fnerrors.InvocationError("vault", "failed to create vault client: %w", err)
		}
		defer client.Close()

		resp, err := client.Vault.ListObjects(ctx, &v1beta.ListObjectsRequest{})
		if err != nil {
			return fnerrors.InvocationError("vault", "failed to list vault objects: %w", err)
		}

		switch *output {
		case "json":
			var b bytes.Buffer
			fmt.Fprint(&b, "[")
			for k, obj := range resp.Objects {
				if k > 0 {
					fmt.Fprint(&b, ",")
				}

				bb, err := protojson.MarshalOptions{UseProtoNames: true, Multiline: true}.Marshal(obj)
				if err != nil {
					return err
				}

				fmt.Fprintf(&b, "\n%s", bb)
			}
			fmt.Fprint(&b, "\n]\n")

			console.Stdout(ctx).Write(b.Bytes())

			return nil
		case "table":
			return printObjectsTable(ctx, resp.Objects)
		default:
			return fnerrors.BadInputError("invalid output format: %s", *output)
		}
	})

	return cmd
}

func NewAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a secret to the vault.",
		Args:  cobra.NoArgs,
	}

	description := cmd.Flags().StringP("description", "d", "", "Description of the secret")
	revealable := cmd.Flags().Bool("revealable", false, "If set, the secret value can be retrieved in future calls")
	labels := cmd.Flags().StringToString("label", nil, "Key-value labels to attach to the secret")
	output := cmd.Flags().StringP("output", "o", "table", "Output format: table, json")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		value, err := tui.AskSecret(ctx, "Secret value", "The secret value to add.", "Secret value")
		if err != nil {
			return err
		}

		tokenSource, err := auth.LoadDefaults()
		if err != nil {
			return fnerrors.InvocationError("vault", "failed to get authentication token: %w", err)
		}

		client, err := vault.NewClient(ctx, tokenSource)
		if err != nil {
			return fnerrors.InvocationError("vault", "failed to create vault client: %w", err)
		}
		defer client.Close()

		// Convert string labels to protobuf labels
		var protoLabels []*stdlib.Label
		for key, value := range *labels {
			protoLabels = append(protoLabels, &stdlib.Label{
				Name:  key,
				Value: value,
			})
		}

		req := &v1beta.CreateObjectRequest{
			Description: *description,
			Value:       string(value),
			Revealable:  *revealable,
			Labels:      protoLabels,
		}

		metadata, err := client.Vault.CreateObject(ctx, req)
		if err != nil {
			return fnerrors.InvocationError("vault", "failed to create vault object: %w", err)
		}

		switch *output {
		case "json":
			bb, err := protojson.MarshalOptions{UseProtoNames: true, Multiline: true}.Marshal(metadata)
			if err != nil {
				return err
			}

			fmt.Fprintln(console.Stdout(ctx), string(bb))
			return nil

		case "table":
			return printObjectMetadata(ctx, metadata)

		default:
			return fnerrors.BadInputError("invalid output format: %s", *output)
		}
	})

	return cmd
}

func NewSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Update an existing secret in the vault.",
		Args:  cobra.NoArgs,
	}

	secretId := cmd.Flags().String("object_id", "", "The object to update.")
	version := cmd.Flags().String("if-version-matches", "", "Only update if the object version matches this value")
	output := cmd.Flags().StringP("output", "o", "table", "Output format: table, json")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		value, err := tui.AskSecret(ctx, "Secret value", "The secret value to add.", "Secret value")
		if err != nil {
			return err
		}

		tokenSource, err := auth.LoadDefaults()
		if err != nil {
			return fnerrors.InvocationError("vault", "failed to get authentication token: %w", err)
		}

		client, err := vault.NewClient(ctx, tokenSource)
		if err != nil {
			return fnerrors.InvocationError("vault", "failed to create vault client: %w", err)
		}
		defer client.Close()

		req := &v1beta.UpdateObjectRequest{
			ObjectId:         *secretId,
			NewValue:         string(value),
			IfVersionMatches: *version,
		}

		metadata, err := client.Vault.UpdateObject(ctx, req)
		if err != nil {
			return fnerrors.InvocationError("vault", "failed to update vault object: %w", err)
		}

		switch *output {
		case "json":
			bb, err := protojson.MarshalOptions{UseProtoNames: true, Multiline: true}.Marshal(metadata)
			if err != nil {
				return err
			}

			fmt.Fprintln(console.Stdout(ctx), string(bb))
			return nil

		case "table":
			return printObjectMetadata(ctx, metadata)

		default:
			return fnerrors.BadInputError("invalid output format: %s", *output)
		}
	})

	return cmd
}

func NewDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an object from the vault.",
		Args:  cobra.NoArgs,
	}

	secretId := cmd.Flags().String("object_id", "", "The object to update.")
	version := cmd.Flags().String("if-version-matches", "", "Only delete if the object version matches this value")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		tokenSource, err := auth.LoadDefaults()
		if err != nil {
			return fnerrors.InvocationError("vault", "failed to get authentication token: %w", err)
		}

		client, err := vault.NewClient(ctx, tokenSource)
		if err != nil {
			return fnerrors.InvocationError("vault", "failed to create vault client: %w", err)
		}
		defer client.Close()

		req := &v1beta.DeleteObjectRequest{
			ObjectId:         *secretId,
			IfVersionMatches: *version,
		}

		_, err = client.Vault.DeleteObject(ctx, req)
		if err != nil {
			return fnerrors.InvocationError("vault", "failed to delete vault object: %w", err)
		}

		fmt.Fprintf(console.Stdout(ctx), "Successfully deleted object %s\n", *secretId)
		return nil
	})

	return cmd
}

func printObjectsTable(ctx context.Context, objects []*v1beta.VaultObjectMetadata) error {
	if len(objects) == 0 {
		fmt.Fprintf(console.Stdout(ctx), "No objects found in vault.\n")
		return nil
	}

	fmt.Fprintf(console.Stdout(ctx), "%-36s %-20s %-50s %-20s\n", "OBJECT ID", "VERSION", "DESCRIPTION", "CREATED")
	fmt.Fprintf(console.Stdout(ctx), "%s\n", strings.Repeat("-", 126))

	for _, obj := range objects {
		createdAt := ""
		if obj.CreatedAt != nil {
			createdAt = obj.CreatedAt.AsTime().Format(time.RFC3339)
		}

		description := obj.Description
		if len(description) > 47 {
			description = description[:47] + "..."
		}

		fmt.Fprintf(console.Stdout(ctx), "%-36s %-20s %-50s %-20s\n",
			obj.ObjectId,
			obj.Version,
			description,
			createdAt,
		)
	}

	return nil
}

func printObjectMetadata(ctx context.Context, metadata *v1beta.VaultObjectMetadata) error {
	fmt.Fprintf(console.Stdout(ctx), "Object ID:    %s\n", metadata.ObjectId)
	fmt.Fprintf(console.Stdout(ctx), "Description:  %s\n", metadata.Description)
	fmt.Fprintf(console.Stdout(ctx), "Version:      %s\n", metadata.Version)

	if metadata.CreatedAt != nil {
		fmt.Fprintf(console.Stdout(ctx), "Created At:   %s\n", metadata.CreatedAt.AsTime().Format(time.RFC3339))
	}

	if len(metadata.Labels) > 0 {
		fmt.Fprintf(console.Stdout(ctx), "Labels:\n")
		for _, label := range metadata.Labels {
			fmt.Fprintf(console.Stdout(ctx), "  %s: %s\n", label.Name, label.Value)
		}
	}

	return nil
}
