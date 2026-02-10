// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	v1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/iam/v1beta"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/cmd/token"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func NewTrustRelationshipsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trust-relationships",
		Short: "Manage trust relationships for authentication.",
	}

	cmd.AddCommand(newTrustAddCmd())
	cmd.AddCommand(newTrustListCmd())
	cmd.AddCommand(newTrustRemoveCmd())

	return cmd
}

func newTrustAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new trust relationship.",
		Long:  `Add a new trust relationship by specifying issuer and subject match patterns.`,
		Args:  cobra.NoArgs,
	}

	issuer := cmd.Flags().String("issuer", "", "Token issuer (required).")
	subjectMatch := cmd.Flags().String("subject-match", "", "Subject match pattern (required).")
	audience := cmd.Flags().String("audience", "", "Expected audience value.")
	defaultPermissions := cmd.Flags().StringArray("grant", nil, `Grant default permission as JSON object (can be specified multiple times). Format: {"resource_type":"...","resource_id":"...","actions":["..."]}`)
	defaultTokenDuration := cmd.Flags().String("default_token_duration", "", `Default token duration (e.g. "3600s").`)

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		if *issuer == "" {
			return fnerrors.Newf("--issuer is required")
		}

		if *subjectMatch == "" {
			return fnerrors.Newf("--subject-match is required")
		}

		permissions, err := token.ParseGrants(*defaultPermissions)
		if err != nil {
			return err
		}

		return addTrustRelationship(ctx, *issuer, *subjectMatch, *audience, permissions, *defaultTokenDuration)
	})
}

func newTrustListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List existing trust relationships.",
		Long:  "List trust relationships configured for the current tenant.",
		Args:  cobra.NoArgs,
	}

	output := cmd.Flags().StringP("output", "o", "", "Output format: json")

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		return listTrustRelationships(ctx, *output)
	})
}

func newTrustRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a trust relationship.",
		Long:  "Remove a trust relationship by specifying its ID.",
		Args:  cobra.NoArgs,
	}

	id := cmd.Flags().String("id", "", "ID of the trust relationship to remove (required).")

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		if *id == "" {
			return fnerrors.Newf("--id is required")
		}

		return removeTrustRelationship(ctx, *id)
	})
}

func addTrustRelationship(ctx context.Context, issuer, subjectMatch, audience string, defaultPermissions []*v1beta.Permission, defaultTokenDuration string) error {
	current, err := fnapi.ListTrustRelationships(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(console.Stderr(ctx), "Adding trust relationship...\n")
	fmt.Fprintf(console.Stderr(ctx), "Issuer: %s\n", issuer)
	fmt.Fprintf(console.Stderr(ctx), "Subject Match: %s\n", subjectMatch)

	newTrustRelationship := fnapi.StoredTrustRelationship{
		Issuer:               issuer,
		SubjectMatch:         subjectMatch,
		Audience:             audience,
		DefaultPermissions:   defaultPermissions,
		DefaultTokenDuration: defaultTokenDuration,
	}

	updatedTrustRelationships := append(current.TrustRelationships, newTrustRelationship)

	if err := fnapi.UpdateTrustRelationships(ctx, current.Generation, updatedTrustRelationships); err != nil {
		return err
	}

	fmt.Fprintf(console.Stdout(ctx), "Successfully added trust relationship.\n")
	return nil
}

func listTrustRelationships(ctx context.Context, output string) error {
	response, err := fnapi.ListTrustRelationships(ctx)
	if err != nil {
		return err
	}

	if output == "json" {
		bb, err := json.MarshalIndent(response.TrustRelationships, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(console.Stdout(ctx), string(bb))
		return nil
	}

	fmt.Fprintf(console.Stdout(ctx), "Trust Relationships:\n\n")

	if len(response.TrustRelationships) == 0 {
		fmt.Fprintf(console.Stdout(ctx), "No trust relationships configured.\n")
		return nil
	}

	for _, tr := range response.TrustRelationships {
		fmt.Fprintf(console.Stdout(ctx), "ID: %s\n", tr.Id)
		fmt.Fprintf(console.Stdout(ctx), "  Issuer: %s\n", tr.Issuer)
		fmt.Fprintf(console.Stdout(ctx), "  Subject Match: %s\n", tr.SubjectMatch)
		if tr.Audience != "" {
			fmt.Fprintf(console.Stdout(ctx), "  Audience: %s\n", tr.Audience)
		}
		if len(tr.DefaultPermissions) > 0 {
			fmt.Fprintf(console.Stdout(ctx), "  Default Permissions:\n")
			for _, p := range tr.DefaultPermissions {
				fmt.Fprintf(console.Stdout(ctx), "    - %s: %s", p.ResourceType, strings.Join(p.Actions, ", "))
				if p.ResourceId != "" {
					fmt.Fprintf(console.Stdout(ctx), " (resource_id: %s)", p.ResourceId)
				}
				fmt.Fprintf(console.Stdout(ctx), "\n")
			}
		}
		if tr.DefaultTokenDuration != "" {
			fmt.Fprintf(console.Stdout(ctx), "  Default Token Duration: %s\n", tr.DefaultTokenDuration)
		}
		if tr.CreatedAt != nil {
			fmt.Fprintf(console.Stdout(ctx), "  Created At: %s\n", tr.CreatedAt.Format("2006-01-02 15:04:05 UTC"))
		}
		if tr.CreatorJson != "" {
			fmt.Fprintf(console.Stdout(ctx), "  Creator: %s\n", tr.CreatorJson)
		}
		fmt.Fprintf(console.Stdout(ctx), "\n")
	}

	return nil
}

func removeTrustRelationship(ctx context.Context, id string) error {
	// Get current trust relationships
	current, err := fnapi.ListTrustRelationships(ctx)
	if err != nil {
		return err
	}

	// Find and remove the trust relationship with the specified ID
	var updatedTrustRelationships []fnapi.StoredTrustRelationship
	var found bool
	var removedTr fnapi.StoredTrustRelationship

	for _, tr := range current.TrustRelationships {
		if tr.Id == id {
			found = true
			removedTr = tr
		} else {
			updatedTrustRelationships = append(updatedTrustRelationships, tr)
		}
	}

	if !found {
		return fnerrors.Newf("trust relationship with ID %q not found", id)
	}

	fmt.Fprintf(console.Stderr(ctx), "Removing trust relationship...\n")
	fmt.Fprintf(console.Stderr(ctx), "ID: %s\n", removedTr.Id)
	fmt.Fprintf(console.Stderr(ctx), "Issuer: %s\n", removedTr.Issuer)
	fmt.Fprintf(console.Stderr(ctx), "Subject Match: %s\n", removedTr.SubjectMatch)

	// Update trust relationships
	if err := fnapi.UpdateTrustRelationships(ctx, current.Generation, updatedTrustRelationships); err != nil {
		return err
	}

	fmt.Fprintf(console.Stdout(ctx), "Successfully removed trust relationship.\n")
	return nil
}

