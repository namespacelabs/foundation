// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"filippo.io/age"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/digestfs"
	"namespacelabs.dev/foundation/internal/keys"
	"namespacelabs.dev/foundation/workspace/dirs"
)

func NewKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "keys",
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

	reencrypt := false

	encrypt := &cobra.Command{
		Use:   "encrypt",
		Short: "Encrypt a directory (e.g. secrets).",
		Args:  cobra.ExactArgs(1),
		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			return enc(ctx, args[0], fnfs.ReadWriteLocalFS(args[0]), reencrypt)
		}),
	}

	importCmd := &cobra.Command{
		Use:   "import [public-key]",
		Short: "Import an existing public/private key pair.",
		Args:  cobra.ExactArgs(1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			return importImpl(ctx, args[0])
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

			origSecretsDir := args[0]
			archive, err := os.Open(filepath.Join(origSecretsDir, keys.EncryptedFile))
			if err != nil {
				return fnerrors.InternalError("failed to open encrypted file: %w", err)
			}
			defer archive.Close()

			fsys, err := keys.DecryptAsFS(ctx, keyDir, archive)
			if err != nil {
				return fnerrors.InternalError("failed to decrypt: %w", err)
			}

			originalDigest, err := digestfs.Digest(ctx, fsys)
			if err != nil {
				return fnerrors.InternalError("failed to compute a digest of the input: %w", err)
			}

			// XXX guarantee in-memory?
			tmpDirPath, err := dirs.CreateUserTempDir(origSecretsDir, "keys-shell")
			if err != nil {
				return fnerrors.InternalError("failed to create tempdir: %w", err)
			}

			defer os.RemoveAll(tmpDirPath)

			tmpDir := fnfs.ReadWriteLocalFS(tmpDirPath)

			if err := fnfs.VisitFiles(ctx, fsys, func(path string, contents bytestream.ByteStream, dirent fs.DirEntry) error {
				d := filepath.Dir(path)
				if d != "." {
					if err := os.Mkdir(filepath.Join(tmpDirPath, d), 0700); err != nil {
						return fnerrors.InternalError("%s: failed to mkdir: %w", path, err)
					}
				}

				return fnfs.WriteByteStream(ctx, tmpDir, path, contents, 0600)
			}); err != nil {
				return fnerrors.InternalError("visitfiles failed: %w", err)
			}

			if err := func() error {
				done := console.EnterInputMode(ctx)
				defer done()

				bash := exec.CommandContext(ctx, "bash")
				bash.Stdout = os.Stdout
				bash.Stderr = os.Stderr
				bash.Stdin = os.Stdin
				bash.Dir = tmpDirPath

				return bash.Run()
			}(); err != nil {
				return err
			}

			changedDigest, err := digestfs.Digest(ctx, tmpDir)
			if err == nil {
				// If we fail to compute the digest, it's ok, just go ahead and rewrite the contents.
				if changedDigest == originalDigest {
					fmt.Fprintf(console.Stdout(ctx), "No changes were made to %s.\n", archive.Name())
					return nil
				}
			}

			if err := keys.EncryptLocal(ctx, fnfs.ReadWriteLocalFS(origSecretsDir), tmpDir); err != nil {
				return err
			}

			fmt.Fprintf(console.Stdout(ctx), "Updated %q.\n", archive.Name())

			return nil
		}),
	}

	encrypt.Flags().BoolVar(&reencrypt, "reencrypt", reencrypt, "Use re-encryption instead.")

	cmd.AddCommand(list)
	cmd.AddCommand(generate)
	cmd.AddCommand(encrypt)
	cmd.AddCommand(importCmd)
	cmd.AddCommand(shell)

	return cmd
}

func enc(ctx context.Context, dir string, src fs.FS, reencrypt bool) error {
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

func readPassword(ctx context.Context, prompt string) ([]byte, error) {
	if term.IsTerminal(syscall.Stdin) {
		done := console.EnterInputMode(ctx, prompt)
		defer done()
		pass, err := term.ReadPassword(syscall.Stdin)
		if err != nil {
			return nil, err
		}
		return pass, nil
	} else {
		reader := bufio.NewReader(os.Stdin)
		// Read until (required) newline.
		s, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		return []byte(s), nil
	}
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

	pass, err := readPassword(ctx, "Please paste the private key (the input will not be echo-ed):\n")
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
