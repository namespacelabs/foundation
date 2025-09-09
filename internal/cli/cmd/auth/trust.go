// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
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

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		if *issuer == "" {
			return fnerrors.Newf("--issuer is required")
		}

		if *subjectMatch == "" {
			return fnerrors.Newf("--subject-match is required")
		}

		return addTrustRelationship(ctx, *issuer, *subjectMatch)
	})
}

func newTrustListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List existing trust relationships.",
		Long:  "List trust relationships configured for the current tenant.",
		Args:  cobra.NoArgs,
	}

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		return listTrustRelationships(ctx)
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

func addTrustRelationship(ctx context.Context, issuer, subjectMatch string) error {
	// Get current trust relationships
	current, err := fnapi.ListTrustRelationships(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(console.Stderr(ctx), "Adding trust relationship...\n")
	fmt.Fprintf(console.Stderr(ctx), "Issuer: %s\n", issuer)
	fmt.Fprintf(console.Stderr(ctx), "Subject Match: %s\n", subjectMatch)

	// Create new trust relationship (ID and CreatorJson will be set server-side)
	newTrustRelationship := fnapi.StoredTrustRelationship{
		Issuer:       issuer,
		SubjectMatch: subjectMatch,
	}

	// Add to existing trust relationships
	updatedTrustRelationships := append(current.TrustRelationships, newTrustRelationship)

	// Update trust relationships
	if err := fnapi.UpdateTrustRelationships(ctx, current.Generation, updatedTrustRelationships); err != nil {
		return err
	}

	fmt.Fprintf(console.Stdout(ctx), "Successfully added trust relationship.\n")
	return nil
}

func listTrustRelationships(ctx context.Context) error {
	response, err := fnapi.ListTrustRelationships(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(console.Stdout(ctx), "Trust Relationships (Generation: %s):\n\n", response.Generation)

	if len(response.TrustRelationships) == 0 {
		fmt.Fprintf(console.Stdout(ctx), "No trust relationships configured.\n")
		return nil
	}

	for _, tr := range response.TrustRelationships {
		fmt.Fprintf(console.Stdout(ctx), "ID: %s\n", tr.Id)
		fmt.Fprintf(console.Stdout(ctx), "  Issuer: %s\n", tr.Issuer)
		fmt.Fprintf(console.Stdout(ctx), "  Subject Match: %s\n", tr.SubjectMatch)
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
