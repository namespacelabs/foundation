// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/universe/db/postgres"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/go-ids"
)

func NewUseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "use",
		Short: "Use is a set of dependency controlled commands which can be used to manage your resources.",
	}

	// TODO: this, and other commands, should be dynamically discovered. See #414.
	h := hydrateArgs{envRef: "dev", rehydrateOnly: true}

	var database string
	psql := &cobra.Command{
		Use:   "psql",
		Short: "Start a Postgres SQL shell for the specified server.",
		Args:  cobra.MaximumNArgs(1),
		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			res, err := h.ComputeStack(ctx, args)
			if err != nil {
				return err
			}

			if len(res.Focus) != 1 {
				return fnerrors.New("psql takes exactly one server, not more, not less")
			}

			config, err := determineConfiguration(res)
			if err != nil {
				return err
			}

			if database == "" {
				if len(config.Instantiated) == 1 && len(config.Instantiated[0].Database) == 1 {
					database = config.Instantiated[0].Database[0].Name
				}
			}

			dbIndex := map[string]*postgres.Database{}
			credsIndex := map[string]*postgres.InstantiatedDatabase_Credentials{}
			names := uniquestrings.List{}
			for _, n := range config.Instantiated {
				for _, db := range n.Database {
					dbIndex[db.Name] = db
					credsIndex[db.Name] = n.Credentials
					names.Add(db.Name)
				}
			}

			if database == "" {
				return fnerrors.UsageError(fmt.Sprintf("Try one of the following databases: %s", strings.Join(names.Strings(), ", ")), "No database specified.")
			}

			db, ok := dbIndex[database]
			if !ok {
				return fnerrors.UsageError(fmt.Sprintf("Try one of the following databases: %v", names.Strings()), "Specified database does not exist.")
			}

			creds, ok := credsIndex[database]
			if !ok {
				return fnerrors.BadInputError("%s: no credentials available", database)
			}

			// XXX generalize.
			k8s, err := kubernetes.New(ctx, res.Env.Workspace(), res.Env.DevHost(), res.Env.Proto())
			if err != nil {
				return err
			}

			psqlImage, err := compute.GetValue(ctx, oci.ResolveDigest("postgres:14.3-alpine@sha256:a00af33e23643f497a42bc24d2f6f28cc67f3f48b076135c5626b2e07945ff9c"))
			if err != nil {
				return err
			}

			runOpts := runtime.ServerRunOpts{
				WorkingDir: "/",
				Image:      psqlImage,
				Command:    []string{"psql"},
				Args: []string{
					"-h", db.HostedAt.Address,
					"-p", fmt.Sprintf("%d", db.HostedAt.Port),
					db.Name, "postgres",
				},
				Env: []*schema.BinaryConfig_EnvEntry{
					{
						Name:                   "PGPASSWORD",
						ExperimentalFromSecret: fmt.Sprintf("%s:%s", creds.SecretResourceName, creds.SecretName),
					},
				},
				ReadOnlyFilesystem: true,
			}

			return k8s.RunAttached(ctx, "psql-"+ids.NewRandomBase32ID(8), runOpts, runtime.TerminalIO{
				Stdin:  os.Stdin,
				Stdout: os.Stdout,
				Stderr: os.Stderr,
			})
		}),
	}

	psql.Flags().StringVar(&database, "database", "", "Connect to the specified database.")

	h.Configure(psql)

	cmd.AddCommand(psql)

	return cmd
}

func determineConfiguration(res *hydrateResult) (*postgres.InstantiatedDatabases, error) {
	for _, computed := range res.Rehydrated.ComputedConfigs.GetEntry() {
		if computed.ServerPackage == res.Focus[0].String() {
			for _, entry := range computed.Configuration {
				c := &postgres.InstantiatedDatabases{}
				if entry.Impl.MessageIs(c) {
					if err := entry.Impl.UnmarshalTo(c); err != nil {
						return nil, err
					}

					return c, nil
				}
			}
		}
	}

	return nil, nil
}
