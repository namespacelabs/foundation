// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package terminal

import (
	"bufio"
	"context"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/go-ids"
)

func newCreateCmd() *cobra.Command {
	run := &cobra.Command{
		Use:   "create",
		Short: "Creates a new instance that can be used as a terminal.",
		Args:  cobra.NoArgs,
	}

	image := run.Flags().String("image", "", "Which image to run.")
	machineType := run.Flags().String("machine_type", "", "Specify the machine type.")
	duration := run.Flags().Duration("duration", 0, "For how long to run the ephemeral environment.")
	envFile := run.Flags().String("envfile", "", "If specified, passes the contents of the envfile to the container.")
	env := run.Flags().StringToStringP("env", "e", map[string]string{}, "Pass these additional environment variables to the container.")

	run.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *image == "" {
			return fnerrors.New("--image is required")
		}

		opts := cluster.CreateContainerOpts{
			Name:            ids.NewRandomBase32ID(6),
			Image:           *image,
			Args:            []string{"sleep", "infinity"},
			EnableDocker:    true,
			ForwardNscState: true,
			Features:        []string{"EXP_USE_CONTAINER_AS_TERMINAL_SOURCE"},
		}

		if *envFile != "" {
			parsed, err := parseEnvFile(*envFile)
			if err != nil {
				return err
			}

			opts.Env = parsed
		}

		if len(*env) > 0 {
			if opts.Env == nil {
				opts.Env = map[string]string{}
			}

			maps.Copy(opts.Env, *env)
		}

		resp, err := cluster.CreateContainerInstance(ctx, *machineType, *duration, "", false, opts)
		if err != nil {
			return err
		}

		return cluster.PrintCreateContainersResult(ctx, "plain", resp)
	})

	return run
}

func parseEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fnerrors.New("failed to open env file %s: %w", path, err)
	}

	defer f.Close()

	vars := map[string]string{}

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		// skip comment lines and empty line
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}

		arr := strings.SplitN(line, "=", 2)
		if arr[0] == "" || len(arr) != 2 {
			return nil, fnerrors.New("invalid environment variable: %q", line)
		}

		vars[arr[0]] = arr[1]
	}

	if err = sc.Err(); err != nil {
		return nil, err
	}

	return vars, nil
}
