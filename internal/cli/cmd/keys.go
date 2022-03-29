// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"filippo.io/age"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/keys"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func NewKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
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

	reencrypt := false

	encrypt := &cobra.Command{
		Use:   "encrypt",
		Short: "Encrypt a directory (e.g. secrets).",
		Args:  cobra.ExactArgs(1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			return enc(ctx, args[0], fnfs.ReadWriteLocalFS(args[0]), reencrypt)
		}),
	}

	shell := &cobra.Command{
		Use:   "shell",
		Short: "Spawns a shell with the decrypted contents, allowing changes, which are then encrypted.",
		Args:  cobra.ExactArgs(1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			keyDir, err := keys.KeysDir()
			if err != nil {
				return fnerrors.InternalError("failed to fetch keydir: %w", err)
			}

			archive, err := os.Open(filepath.Join(args[0], keys.EncryptedFile))
			if err != nil {
				return fnerrors.InternalError("failed to open encrypted file: %w", err)
			}
			defer archive.Close()

			fsys, err := keys.DecryptAsFS(ctx, keyDir, archive)
			if err != nil {
				return fnerrors.InternalError("failed to decrypt: %w", err)
			}

			// XXX guarantee in-memory?
			dir, err := os.MkdirTemp(args[0], "keys-shell")
			if err != nil {
				return fnerrors.InternalError("failed to create tempdir: %w", err)
			}

			defer os.RemoveAll(dir)

			dst := fnfs.ReadWriteLocalFS(dir)

			if err := fnfs.VisitFiles(ctx, fsys, func(path string, contents []byte, dirent fs.DirEntry) error {
				d := filepath.Dir(path)
				if d != "." {
					if err := os.Mkdir(filepath.Join(dir, d), 0700); err != nil {
						return fnerrors.InternalError("%s: failed to mkdir: %w", path, err)
					}
				}

				return fnfs.WriteFile(ctx, dst, path, contents, 0600)
			}); err != nil {
				return fnerrors.InternalError("visitfiles failed: %w", err)
			}

			if err := func() error {
				done := tasks.EnterInputMode(ctx)
				defer done()

				bash := exec.CommandContext(ctx, "bash")
				bash.Stdout = os.Stdout
				bash.Stderr = os.Stderr
				bash.Stdin = os.Stdin
				bash.Dir = dir

				return bash.Run()
			}(); err != nil {
				return err
			}

			return enc(ctx, args[0], dst, false)
		}),
	}

	encrypt.Flags().BoolVar(&reencrypt, "reencrypt", reencrypt, "Use re-encryption instead.")

	cmd.AddCommand(generate)
	cmd.AddCommand(encrypt)
	cmd.AddCommand(shell)

	return cmd
}

func enc(ctx context.Context, dir string, src fs.ReadDirFS, reencrypt bool) error {
	fsys := fnfs.ReadWriteLocalFS(dir)

	if reencrypt {
		if err := keys.Reencrypt(ctx, fsys); err != nil {
			return err
		}
	} else {
		if err := keys.EncryptLocal(ctx, fsys, src); err != nil {
			return err
		}
	}

	fmt.Fprintf(console.Stdout(ctx), "Updated %s\n", filepath.Join(dir, keys.EncryptedFile))
	return nil
}