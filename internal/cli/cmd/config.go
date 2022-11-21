// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

var configKeys = []struct {
	key   string
	label string
}{
	{"telemetry", "Is telemetry enabled"},
}

func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manages `ns`'s configuration.",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			stdout := console.Stdout(ctx)

			for _, kv := range configKeys {
				fmt.Fprintf(stdout, "%s: %v\n", kv.label, viper.GetBool(kv.key))
			}

			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "For more information on telemetry, please visit https://namespace.so/telemetry.")
			fmt.Fprintln(stdout)
			return nil
		}),
	}

	disableCmd := &cobra.Command{
		Use:   "disable",
		Short: "Disables the specified configuration.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			if err := validateConfigKey(key); err != nil {
				return err
			}
			return updateConfiguration(cmd.Context(), func(m map[string]any) error {
				m[key] = false
				return nil
			})
		},
	}

	enableCmd := &cobra.Command{
		Use:   "enable",
		Short: "Enables the specified configuration.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			if err := validateConfigKey(key); err != nil {
				return err
			}
			return updateConfiguration(cmd.Context(), func(m map[string]any) error {
				m[key] = true
				return nil
			})
		},
	}

	cmd.AddCommand(disableCmd)
	cmd.AddCommand(enableCmd)

	return cmd
}

func validateConfigKey(key string) error {
	for _, kv := range configKeys {
		if kv.key == key {
			return nil
		}
	}
	return fnerrors.New("%s: no such configuration key", key)
}

func updateConfiguration(ctx context.Context, update func(map[string]any) error) error {
	fnDir, err := dirs.Config()
	if err != nil {
		return err
	}

	existing, err := os.ReadFile(filepath.Join(fnDir, "config.json"))
	if err != nil {
		if os.IsNotExist(err) {
			existing = []byte("{}")
		} else {
			return err
		}
	}

	var m map[string]any
	if err := json.Unmarshal(existing, &m); err != nil {
		return err
	}

	if err := update(m); err != nil {
		return err
	}

	serialized, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(fnDir, "config.json"), serialized, 0644); err != nil {
		return err
	}

	fmt.Fprintln(console.Stdout(ctx), "Configuration updated.")

	return nil
}
