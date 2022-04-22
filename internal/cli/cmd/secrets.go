// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/keys"
	"namespacelabs.dev/foundation/internal/secrets"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/module"
)

const bundleName = "server.secrets"

func NewSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "secrets",
	}

	list := &cobra.Command{
		Use:   "info",
		Short: "Describes the contents of the specified server's secrets archive.",
		Args:  cobra.MaximumNArgs(1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			_, bundle, err := loadBundleFromArgs(ctx, args, nil)
			if err != nil {
				return err
			}

			out := console.Stdout(ctx)

			switch len(bundle.Readers()) {
			case 0:
				fmt.Fprintln(out, "No readers.")

			default:
				fmt.Fprintln(out, "Readers:")
				for _, r := range bundle.Readers() {
					fmt.Fprintf(out, "  %s", r.PublicKey)
					if r.Description != "" {
						fmt.Fprintf(out, "  # %s", r.Description)
					}
					fmt.Fprintln(out)
				}
			}

			switch len(bundle.Definitions()) {
			case 0:
				fmt.Fprintln(out, "No definitions.")

			default:
				fmt.Fprintln(out, "Definitions:")
				for _, def := range bundle.Definitions() {
					fmt.Fprintf(out, "  %s:%s", def.Key.PackageName, def.Key.Key)
					if def.Key.SecondaryKey != "" {
						fmt.Fprintf(out, " (%s)", def.Key.SecondaryKey)
					}
					fmt.Fprintln(out)
				}
			}

			return nil
		}),
	}

	var secretKey, keyID string
	var rawtext bool

	set := &cobra.Command{
		Use:   "set",
		Short: "Sets the specified secret value.",
		Args:  cobra.MaximumNArgs(1),
		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			loc, bundle, err := loadBundleFromArgs(ctx, args, func(ctx context.Context) (*secrets.Bundle, error) {
				return secrets.NewBundle(ctx, keyID)
			})
			if err != nil {
				return err
			}

			packageName, key, err := parseKey(secretKey)
			if err != nil {
				return err
			}

			if _, err := workspace.NewPackageLoader(loc.root).LoadByName(ctx, schema.PackageName(packageName)); err != nil {
				return err
			}

			value := readLine(ctx, fmt.Sprintf("Specify a value for %q in %s.\n\nValue: ", key, packageName))
			if value == "" {
				return errors.New("no value provided, skipping")
			}

			bundle.Set(packageName, key, []byte(value))

			return writeBundle(ctx, loc, bundle, !rawtext)
		}),
	}

	delete := &cobra.Command{
		Use:   "delete",
		Short: "Deletes the specified secret value.",
		Args:  cobra.MaximumNArgs(1),
		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			loc, bundle, err := loadBundleFromArgs(ctx, args, nil)
			if err != nil {
				return err
			}

			packageName, key, err := parseKey(secretKey)
			if err != nil {
				return err
			}

			if !bundle.Delete(packageName, key) {
				return errors.New("no such key")
			}

			return writeBundle(ctx, loc, bundle, !rawtext)
		}),
	}

	addReader := &cobra.Command{
		Use:   "add-reader",
		Short: "Adds a receipient to a secret bundle.",
		Args:  cobra.MaximumNArgs(1),
		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			loc, bundle, err := loadBundleFromArgs(ctx, args, nil)
			if err != nil {
				return err
			}

			if err := bundle.EnsureReader(keyID); err != nil {
				return err
			}

			return writeBundle(ctx, loc, bundle, !rawtext)
		}),
	}

	set.Flags().StringVar(&secretKey, "secret", "", "The secret key, in {package_name}:{name} format.")
	set.Flags().StringVar(&keyID, "key", "", "Use this specific key identity when creating a new bundle.")
	set.Flags().BoolVar(&rawtext, "rawtext", rawtext, "If set to true, the bundle is not encrypted (use for testing purposes only).")
	_ = set.MarkFlagRequired("secret")

	delete.Flags().StringVar(&secretKey, "secret", "", "The secret key, in {package_name}:{name} format.")
	delete.Flags().BoolVar(&rawtext, "rawtext", rawtext, "If set to true, the bundle is not encrypted (use for testing purposes only).")
	_ = delete.MarkFlagRequired("secret")

	addReader.Flags().StringVar(&keyID, "key", "", "The key to add to the bundle.")
	addReader.Flags().BoolVar(&rawtext, "rawtext", rawtext, "If set to true, the bundle is not encrypted (use for testing purposes only).")
	_ = addReader.MarkFlagRequired("key")

	cmd.AddCommand(list)
	cmd.AddCommand(set)
	cmd.AddCommand(delete)
	cmd.AddCommand(addReader)

	return cmd
}

type createFunc func(context.Context) (*secrets.Bundle, error)

type location struct {
	root *workspace.Root
	loc  fnfs.Location
}

func loadBundleFromArgs(ctx context.Context, args []string, createIfMissing createFunc) (*location, *secrets.Bundle, error) {
	root, loc, err := module.PackageAtArgs(ctx, args)
	if err != nil {
		return nil, nil, err
	}

	pkg, err := workspace.NewPackageLoader(root).LoadByName(ctx, loc.AsPackageName())
	if err != nil {
		return nil, nil, err
	}

	if pkg.Server == nil {
		return nil, nil, fnerrors.BadInputError("%s: expected a server", loc.AsPackageName())
	}

	contents, err := fs.ReadFile(pkg.Location.Module.ReadWriteFS(), pkg.Location.Rel(bundleName))
	if err != nil {
		if os.IsNotExist(err) && createIfMissing != nil {
			bundle, err := createIfMissing(ctx)
			return &location{root, loc}, bundle, err
		}

		return nil, nil, err
	}

	keyDir, err := keys.KeysDir()
	if err != nil {
		return nil, nil, err
	}

	bundle, err := secrets.LoadBundle(ctx, keyDir, contents)
	return &location{root, loc}, bundle, err
}

func readLine(ctx context.Context, prompt string) string {
	done := console.EnterInputMode(ctx, prompt)
	defer done()

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}

	return ""
}

func parseKey(v string) (string, string, error) {
	parts := strings.SplitN(v, ":", 2)
	if len(parts) < 2 {
		return "", "", fnerrors.BadInputError("invalid secret key definition, expected {package_name}:{name}")
	}

	return parts[0], parts[1], nil
}

func writeBundle(ctx context.Context, loc *location, bundle *secrets.Bundle, encrypt bool) error {
	return fnfs.WriteWorkspaceFile(ctx, loc.root.FS(), loc.loc.Rel(bundleName), func(w io.Writer) error {
		return bundle.SerializeTo(ctx, w, encrypt)
	})
}
