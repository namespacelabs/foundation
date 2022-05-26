// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/kr/text"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/keys"
	"namespacelabs.dev/foundation/internal/secrets"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/module"
)

const bundleName = "server.secrets"

func NewSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "Manage secrets for a given server.",
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

			bundle.DescribeTo(console.Stdout(ctx))
			return nil
		}),
	}

	var secretKey, keyID, fromFile, specificEnv string
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

			key, err := parseKey(secretKey)
			if err != nil {
				return err
			}

			if specificEnv != "" {
				if _, err := provision.RequireEnv(loc.root, specificEnv); err != nil {
					return err
				}

				key.EnvironmentName = specificEnv
			}

			if _, err := workspace.NewPackageLoader(loc.root).LoadByName(ctx, schema.PackageName(key.PackageName)); err != nil {
				return err
			}

			var value []byte
			if fromFile != "" {
				value, err = ioutil.ReadFile(fromFile)
				if err != nil {
					return fnerrors.BadInputError("%s: failed to load: %w", fromFile, err)
				}
			} else {
				valueStr, err := tui.Ask(ctx, "Set a new secret value", fmt.Sprintf("Package: %s\nKey: %q\n\n%s", key.PackageName, key.Key, lipgloss.NewStyle().Faint(true).Render("Note: for multi-line input, use the --from_file flag.")), "Value")
				if err != nil {
					return err
				}
				if valueStr == "" {
					return fnerrors.New("no value provided, skipping")
				}
				value = []byte(valueStr)
			}

			bundle.Set(key, value)

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

			key, err := parseKey(secretKey)
			if err != nil {
				return err
			}

			if !bundle.Delete(key.PackageName, key.Key) {
				return fnerrors.New("no such key")
			}

			return writeBundle(ctx, loc, bundle, !rawtext)
		}),
	}

	reveal := &cobra.Command{
		Use:   "reveal",
		Short: "Reveals the specified secret value.",
		Args:  cobra.MaximumNArgs(1),
		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			_, bundle, err := loadBundleFromArgs(ctx, args, nil)
			if err != nil {
				return err
			}

			key, err := parseKey(secretKey)
			if err != nil {
				return err
			}

			key.EnvironmentName = specificEnv

			results, err := bundle.LookupValues(ctx, key)
			if err != nil {
				return err
			}

			out := console.Stdout(ctx)

			if len(results) == 1 && utf8.Valid(results[0].Value) {
				fmt.Fprintf(out, "%s\n", results[0].Value)
				return nil
			}

			for k, result := range results {
				if k > 0 {
					fmt.Fprintln(out)
				}

				secrets.DescribeKey(out, result.Key)

				if utf8.Valid(result.Value) {
					fmt.Fprintf(out, "\n\n  %s\n", result.Value)
				} else {
					fmt.Fprintf(out, " raw value:\n\n")
					if err := secrets.OutputBase64(text.NewIndentWriter(out, []byte("  ")), result.Value); err != nil {
						return err
					}
				}
			}

			return nil
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
	set.Flags().StringVar(&fromFile, "from_file", "", "Load the file contents as the secret value.")
	set.Flags().BoolVar(&rawtext, "rawtext", rawtext, "If set to true, the bundle is not encrypted (use for testing purposes only).")
	set.Flags().StringVar(&specificEnv, "env", "", "If set, only sets the specified secret for the named environment (e.g. dev, or prod).")
	_ = set.MarkFlagRequired("secret")

	delete.Flags().StringVar(&secretKey, "secret", "", "The secret key, in {package_name}:{name} format.")
	delete.Flags().BoolVar(&rawtext, "rawtext", rawtext, "If set to true, the bundle is not encrypted (use for testing purposes only).")
	_ = delete.MarkFlagRequired("secret")

	reveal.Flags().StringVar(&secretKey, "secret", "", "The secret key, in {package_name}:{name} format.")
	reveal.Flags().StringVar(&specificEnv, "env", "", "If set, matches specified secret with the named environment (e.g. dev, or prod).")
	_ = reveal.MarkFlagRequired("secret")

	addReader.Flags().StringVar(&keyID, "key", "", "The key to add to the bundle.")
	addReader.Flags().BoolVar(&rawtext, "rawtext", rawtext, "If set to true, the bundle is not encrypted (use for testing purposes only).")
	_ = addReader.MarkFlagRequired("key")

	cmd.AddCommand(list)
	cmd.AddCommand(set)
	cmd.AddCommand(delete)
	cmd.AddCommand(reveal)
	cmd.AddCommand(addReader)

	return cmd
}

type createFunc func(context.Context) (*secrets.Bundle, error)

type location struct {
	root *workspace.Root
	loc  workspace.Location
}

func loadBundleFromArgs(ctx context.Context, args []string, createIfMissing createFunc) (*location, *secrets.Bundle, error) {
	root, loc, err := module.PackageAtArgs(ctx, args)
	if err != nil {
		return nil, nil, err
	}

	pkg, err := loadPackage(ctx, root, loc, args)
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
			return &location{root, pkg.Location}, bundle, err
		}

		return nil, nil, err
	}

	keyDir, err := keys.KeysDir()
	if err != nil {
		return nil, nil, err
	}

	bundle, err := secrets.LoadBundle(ctx, keyDir, contents)
	return &location{root, pkg.Location}, bundle, err
}

func loadPackage(ctx context.Context, root *workspace.Root, loc fnfs.Location, maybePkg []string) (*workspace.Package, error) {
	pkg, err := workspace.NewPackageLoader(root).LoadByName(ctx, loc.AsPackageName())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && len(maybePkg) > 0 {
			var err2 error
			pkg, err2 = workspace.NewPackageLoader(root).LoadByName(ctx, schema.PackageName(maybePkg[0]))
			if err2 == nil {
				return pkg, nil
			}
		}
		return nil, err
	}
	return pkg, nil
}

func parseKey(v string) (*secrets.ValueKey, error) {
	parts := strings.SplitN(v, ":", 2)
	if len(parts) < 2 {
		return nil, fnerrors.BadInputError("invalid secret key definition, expected {package_name}:{name}")
	}

	return &secrets.ValueKey{PackageName: parts[0], Key: parts[1]}, nil
}

func writeBundle(ctx context.Context, loc *location, bundle *secrets.Bundle, encrypt bool) error {
	return fnfs.WriteWorkspaceFile(ctx, console.Stdout(ctx), loc.root.FS(), loc.loc.Rel(bundleName), func(w io.Writer) error {
		return bundle.SerializeTo(ctx, w, encrypt)
	})
}
