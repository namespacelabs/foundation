// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
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

	cmd.AddCommand(newPsql())
	cmd.AddCommand(newPgdump())
	cmd.AddCommand(newPgrestore())

	return cmd
}

// TODO: this, and other commands, should be dynamically discovered. See #414.
func newPsql() *cobra.Command {
	var res hydrateResult
	var database string

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "psql",
			Short: "Start a Postgres SQL shell for the specified server.",
		}).
		WithFlags(func(cmd *cobra.Command) {
			cmd.Flags().StringVar(&database, "database", "", "Connect to the specified database.")
		}).
		With(parseHydrationWithDeps(&res, &fncobra.ParseLocationsOpts{RequireSingle: true}, &hydrateOpts{rehydrateOnly: true})...).
		Do(func(ctx context.Context) error {
			return runPostgresCmd(ctx, database, &res, func(ctx context.Context, rt kubernetes.K8sRuntime, bind databaseBind, opts runtime.ServerRunOpts) error {
				opts.Command = []string{"psql"}
				opts.Args = []string{
					"-h", bind.Database.HostedAt.Address,
					"-p", fmt.Sprintf("%d", bind.Database.HostedAt.Port),
					bind.Database.Name, "postgres",
				}

				return rt.RunAttached(ctx, "psql-"+ids.NewRandomBase32ID(8), opts, runtime.TerminalIO{
					TTY:    true,
					Stdin:  os.Stdin,
					Stdout: os.Stdout,
					Stderr: os.Stderr,
				})
			})
		})
}

// TODO: this, and other commands, should be dynamically discovered. See #414.
func newPgdump() *cobra.Command {
	var res hydrateResult
	var database string
	var out string

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "pgdump",
			Short: "Performs a dump of the contents of an existing database.",
		}).
		WithFlags(func(cmd *cobra.Command) {
			cmd.Flags().StringVar(&database, "database", "", "Connect to the specified database.")
			cmd.Flags().StringVar(&out, "out", "", "If set, dumps the output to the specified file.")
		}).
		With(parseHydrationWithDeps(&res, &fncobra.ParseLocationsOpts{RequireSingle: true}, &hydrateOpts{rehydrateOnly: true})...).
		Do(func(ctx context.Context) error {
			return runPostgresCmd(ctx, database, &res, func(ctx context.Context, rt kubernetes.K8sRuntime, bind databaseBind, opts runtime.ServerRunOpts) error {
				opts.Command = []string{"pg_dump"}
				opts.Args = []string{
					"-h", bind.Database.HostedAt.Address,
					"-p", fmt.Sprintf("%d", bind.Database.HostedAt.Port),
					"-U", "postgres",
					bind.Database.Name,
				}

				var outw io.Writer
				if out != "" {
					f, err := os.Create(out)
					if err != nil {
						return err
					}
					defer f.Close()
					outw = f
				} else {
					outw = console.Stdout(ctx)
				}

				return rt.RunOneShot(ctx, "pgdump-"+ids.NewRandomBase32ID(8), opts, outw, false)
			})
		})
}

// TODO: this, and other commands, should be dynamically discovered. See #414.
func newPgrestore() *cobra.Command {
	var res hydrateResult
	var database string
	var restore string

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "pgrestore",
			Short: "Performs a restore of the contents of an existing backup.",
		}).
		WithFlags(func(cmd *cobra.Command) {
			cmd.Flags().StringVar(&database, "database", "", "Connect to the specified database.")
			cmd.Flags().StringVar(&restore, "restore", "", "The contents to be restored.")

			_ = cobra.MarkFlagRequired(cmd.Flags(), "restore")
		}).
		With(parseHydrationWithDeps(&res, &fncobra.ParseLocationsOpts{RequireSingle: true}, &hydrateOpts{rehydrateOnly: true})...).
		Do(func(ctx context.Context) error {
			return runPostgresCmd(ctx, database, &res, func(ctx context.Context, rt kubernetes.K8sRuntime, bind databaseBind, opts runtime.ServerRunOpts) error {
				opts.Command = []string{"psql"}
				opts.Args = []string{
					"-h", bind.Database.HostedAt.Address,
					"-p", fmt.Sprintf("%d", bind.Database.HostedAt.Port),
					bind.Database.Name, "postgres",
				}

				f, err := os.Open(restore)
				if err != nil {
					return err
				}

				defer f.Close()

				return rt.RunAttached(ctx, "pgrestore-"+ids.NewRandomBase32ID(8), opts, runtime.TerminalIO{
					Stdin:  f,
					Stdout: os.Stdout,
					Stderr: os.Stderr,
				})
			})
		})
}

type databaseBind struct {
	PackageName string
	Database    *postgres.Database
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

func selectDatabase(ctx context.Context, index map[string]databaseBind, names []string) (string, error) {
	if len(names) == 0 {
		return "", fnerrors.New("no database to connect to")
	}

	var items []databaseItem
	for _, name := range names {
		items = append(items, databaseItem{index[name]})
	}

	item, err := tui.Select(ctx, "Which database to connect to?", items)
	if err != nil {
		return "", err
	}

	if item == nil {
		return "", context.Canceled
	}

	return item.(databaseItem).bind.Database.Name, nil
}

type databaseItem struct {
	bind databaseBind
}

func (d databaseItem) Title() string       { return d.bind.Database.Name }
func (d databaseItem) Description() string { return d.bind.PackageName }
func (d databaseItem) FilterValue() string { return d.bind.Database.Name }

func runPostgresCmd(ctx context.Context, database string, res *hydrateResult, run func(context.Context, kubernetes.K8sRuntime, databaseBind, runtime.ServerRunOpts) error) error {
	config, err := determineConfiguration(res)
	if err != nil {
		return err
	}

	if database == "" {
		if len(config.Instantiated) == 1 && len(config.Instantiated[0].Database) == 1 {
			database = config.Instantiated[0].Database[0].Name
		}
	}

	dbIndex := map[string]databaseBind{}
	credsIndex := map[string]*postgres.InstantiatedDatabase_Credentials{}
	names := uniquestrings.List{}
	for _, n := range config.Instantiated {
		for _, db := range n.Database {
			dbIndex[db.Name] = databaseBind{
				PackageName: n.PackageName,
				Database:    db,
			}
			credsIndex[db.Name] = n.Credentials
			names.Add(db.Name)
		}
	}

	if database == "" {
		database, err = selectDatabase(ctx, dbIndex, names.Strings())
		if err != nil {
			return err
		}
	}

	bind, ok := dbIndex[database]
	if !ok {
		return fnerrors.UsageError(fmt.Sprintf("Try one of the following databases: %v", names.Strings()), "Specified database does not exist.")
	}

	creds, ok := credsIndex[database]
	if !ok {
		return fnerrors.BadInputError("%s: no credentials available", database)
	}

	// XXX generalize.
	k8s, err := kubernetes.NewFromEnv(ctx, res.Env)
	if err != nil {
		return err
	}

	psqlImage, err := compute.GetValue(ctx,
		oci.ResolveDigest("postgres:14.3-alpine@sha256:a00af33e23643f497a42bc24d2f6f28cc67f3f48b076135c5626b2e07945ff9c",
			oci.ResolveOpts{PublicImage: true}).ImageID())
	if err != nil {
		return err
	}

	runOpts := runtime.ServerRunOpts{
		WorkingDir: "/",
		Image:      psqlImage,
		Env: []*schema.BinaryConfig_EnvEntry{
			{
				Name:                   "PGPASSWORD",
				ExperimentalFromSecret: fmt.Sprintf("%s:%s", creds.SecretResourceName, creds.SecretName),
			},
		},
		ReadOnlyFilesystem: true,
	}

	return run(ctx, k8s.Bind(res.Env.Workspace(), res.Env.Proto()), bind, runOpts)

}
