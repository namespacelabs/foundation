package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"namespacelabs.dev/foundation/internal/cache/mode"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func newModeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mode",
		Short: "Compute cache directories for various frameworks, languages & tools.",
	}

	cmd.AddCommand(
		newModeList(),
		newModeOutput(),
		newModeDetect(),
	)

	return cmd
}

func newModeList() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all supported cache modes.",
	}

	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		modes := mode.DefaultProviders().List()

		switch *output {
		case "plain":
			fmt.Fprintf(console.Stdout(ctx), "%s\n", strings.Join(modes, "\n"))

		case "json":
			enc := json.NewEncoder(console.Stdout(ctx))
			enc.SetIndent("", "  ")
			if err := enc.Encode(modes); err != nil {
				return fnerrors.InternalError("failed to encode as JSON output: %w", err)
			}
		}

		return nil
	})

	return cmd
}

func newModeOutput() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "output",
		Short: "Output cache paths for all or specified cache modes.",
	}

	filter := cmd.Flags().StringSliceP("filter", "f", []string{}, "Cache modes to only include.")
	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		results, err := mode.DefaultProviders().Mode(ctx, *filter...)
		if err != nil {
			return err
		}

		switch *output {
		case "plain":
			paths := make([]string, 0, len(results)*2)
			for _, result := range results {
				paths = append(paths, result.Paths...)
			}
			fmt.Fprintf(console.Stdout(ctx), "%s\n", strings.Join(paths, "\n"))

		case "json":
			enc := json.NewEncoder(console.Stdout(ctx))
			enc.SetIndent("", "  ")
			if err := enc.Encode(results); err != nil {
				return fnerrors.InternalError("failed to encode as JSON output: %w", err)
			}
		}

		return nil
	})

	return cmd
}

func newModeDetect() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "detect",
		Short: "Output cache paths based on detected frameworks, languages & tools.",
	}

	filter := cmd.Flags().StringSliceP("filter", "f", []string{}, "Cache modes to only include.")
	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		// TODO: provide working directory flag.
		results, err := mode.DefaultProviders().Detect(ctx, "./", *filter...)
		if err != nil {
			return err
		}

		switch *output {
		case "plain":
			paths := make([]string, 0, len(results)*2)
			for _, result := range results {
				paths = append(paths, result.Paths...)
			}
			fmt.Fprintf(console.Stdout(ctx), "%s\n", strings.Join(paths, "\n"))

		case "json":
			enc := json.NewEncoder(console.Stdout(ctx))
			enc.SetIndent("", "  ")
			if err := enc.Encode(results); err != nil {
				return fnerrors.InternalError("failed to encode as JSON output: %w", err)
			}
		}

		return nil
	})

	return cmd
}
