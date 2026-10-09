// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package credhelper

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

const dockerUsername = "token"

func NewDockerCredHelperStoreCmd(hidden bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "store",
		Short:  "Unimplemented",
		Args:   cobra.NoArgs,
		Hidden: hidden,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		return fnerrors.Newf("not supported")
	})

	return cmd
}

func NewDockerCredHelperGetCmd(hidden bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "get",
		Short:  "Get Workspace's container registry credentials",
		Args:   cobra.NoArgs,
		Hidden: hidden,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		done := console.EnterInputMode(ctx)
		defer done()

		input, err := readStdin()
		if err != nil {
			return fnerrors.Newf("failed to read from stdin: %w", err)
		}
		regURL := string(input)

		resp, err := api.GetImageRegistry(ctx, api.Methods)
		if err != nil {
			return fnerrors.Newf("failed to get nscloud registries: %w", err)
		}

		registries := append(resp.ExtraRegistry, []*api.ImageRegistry{resp.NSCR}...)
		for _, reg := range registries {
			if reg != nil && regURL == reg.EndpointAddress {
				token, err := fnapi.IssueToken(ctx, 8*time.Hour)
				if err != nil {
					return err
				}

				c := credHelperGetOutput{
					ServerURL: reg.EndpointAddress,
					Username:  dockerUsername,
					Secret:    token,
				}

				enc := json.NewEncoder(os.Stdout)
				return enc.Encode(c)
			}
		}

		// Docker-like tools expect the following error string w/o special formatting
		return errors.New("credentials not found in native keychain")
	})

	return cmd
}

func NewDockerCredHelperListCmd(hidden bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "list",
		Short:  "List Workspace's container registry credentials",
		Args:   cobra.NoArgs,
		Hidden: hidden,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		done := console.EnterInputMode(ctx)
		defer done()

		resp, err := api.GetImageRegistry(ctx, api.Methods)
		if err != nil {
			return fnerrors.Newf("failed to get nscloud registries: %w", err)
		}

		registries := append(resp.ExtraRegistry, []*api.ImageRegistry{resp.NSCR}...)
		output := map[string]string{}
		for _, reg := range registries {
			if reg != nil {
				output[reg.EndpointAddress] = dockerUsername
			}
		}

		enc := json.NewEncoder(os.Stdout)
		return enc.Encode(output)
	})

	return cmd
}

func NewDockerCredHelperEraseCmd(hidden bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "erase",
		Short:  "Unimplemented",
		Args:   cobra.NoArgs,
		Hidden: hidden,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		return fnerrors.Newf("not supported")
	})

	return cmd
}

type credHelperGetOutput struct {
	ServerURL string
	Username  string
	Secret    string
}

func readStdin() ([]byte, error) {
	scanner := bufio.NewScanner(os.Stdin)

	buffer := new(bytes.Buffer)
	for scanner.Scan() {
		buffer.Write(scanner.Bytes())
	}

	if err := scanner.Err(); err != nil {
		return nil, err

	}

	return bytes.TrimSpace(buffer.Bytes()), nil
}
