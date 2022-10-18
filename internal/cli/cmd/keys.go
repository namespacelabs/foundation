// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"filippo.io/age"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/keys"
)

func NewKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "Manage keys used for local secret management.",
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "Displays the configured public keys used for local secret management.",
		Args:  cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			keysDir, err := keys.EnsureKeysDir(ctx)
			if err != nil {
				return err
			}

			return keys.Visit(ctx, keysDir, func(xid *age.X25519Identity) error {
				fmt.Fprintf(console.Stdout(ctx), "%s\n", xid.Recipient())
				return nil
			})
		}),
	}

	generate := &cobra.Command{
		Use:   "generate",
		Short: "Generate a new key, used for local secret management.",
		Args:  cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			keysDir, err := keys.EnsureKeysDir(ctx)
			if err != nil {
				return err
			}

			k, err := age.GenerateX25519Identity()
			if err != nil {
				return err
			}

			filename := fmt.Sprintf("%s.txt", k.Recipient())

			f, err := keysDir.OpenWrite(filename, 0600)
			if err != nil {
				return err
			}

			fmt.Fprintf(f, "%s\n", k)
			if err := f.Close(); err != nil {
				return err
			}

			fmt.Fprintf(console.Stdout(ctx), "Public key: %s\n", k.Recipient())

			return nil
		}),
	}

	importCmd := &cobra.Command{
		Use:   "import <public-key>",
		Short: "Import an existing public/private key pair.",
		Args:  cobra.ExactArgs(1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			return importImpl(ctx, args[0])
		}),
	}

	cmd.AddCommand(list)
	cmd.AddCommand(generate)
	cmd.AddCommand(importCmd)

	return cmd
}

func importImpl(ctx context.Context, publicKey string) error {
	if _, err := age.ParseRecipients(strings.NewReader(publicKey)); err != nil {
		return fnerrors.BadInputError("key %q is not valid: %w", publicKey, err)
	}

	keyDir, err := keys.EnsureKeysDir(ctx)
	if err != nil {
		return fnerrors.InternalError("failed to fetch keydir: %w", err)
	}

	// Check if a given key already exists. We should probably ask if overwrite is intended. As for now, bail out.
	if err := keys.Visit(ctx, keyDir, func(xid *age.X25519Identity) error {
		if xid.Recipient().String() == publicKey {
			return fmt.Errorf("key %q already exists, won't be overwritten", publicKey)
		}
		return nil
	}); err != nil {
		return err
	}

	pass, err := tui.AskSecret(ctx, "Please specify the private key to import",
		"The input will not be echo-ed.", "AGE-SECRET-KEY-... (private key)")
	if err != nil {
		return err
	}

	if identities, err := age.ParseIdentities(bytes.NewReader(pass)); err != nil {
		return err
	} else if len(identities) != 1 {
		return fmt.Errorf("expecting one key to be present, got %d", len(identities))
	}

	if err := fnfs.WriteFile(ctx, keyDir, publicKey+".txt", append(pass, '\n'), 0600); err != nil {
		return err
	}

	fmt.Fprintf(console.Stdout(ctx), "Successfully imported key %q\n", publicKey)
	return nil
}
