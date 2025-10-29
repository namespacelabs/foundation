package cache

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"namespacelabs.dev/foundation/internal/cli/fncobra"
)

func newModeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mode",
		Short: "Compute cache directories for various frameworks, languages & tools.",
	}

	cmd.AddCommand(
		newModesSupported(),
		newModeOutput(),
		newModeDetect(),
	)

	return cmd
}

func newModesSupported() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all supported cache modes.",
	}

	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		fmt.Printf("output: %s\n", *output)
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
		fmt.Printf("filter: (%d) %v\n", len(*filter), *filter)
		fmt.Printf("output: %s\n", *output)
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
		fmt.Printf("filter: (%d) %v\n", len(*filter), *filter)
		fmt.Printf("output: %s\n", *output)
		return nil
	})

	return cmd
}
