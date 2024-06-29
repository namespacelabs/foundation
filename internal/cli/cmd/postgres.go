// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/go-ids"
)

// TODO: this, and other commands, should be dynamically discovered. See #414.
func newPsql() *cobra.Command {
	var res hydrateResult
	var database string

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "psql [--database <database-name>]",
			Short: "Start a Postgres SQL shell for the specified server.",
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.StringVar(&database, "database", "", "Connect to the specified database.")
			_ = cobra.MarkFlagRequired(flags, "database")
		}).
		With(parseHydrationWithDeps(&res, &fncobra.ParseLocationsOpts{RequireSingle: true}, &hydrateOpts{rehydrateOnly: true})...).
		Do(func(ctx context.Context) error {
			return runPostgresCommand(ctx, database, &res, func(ctx context.Context, rt runtime.Planner, bind databaseAccessor, opts runtime.ContainerRunOpts) error {
				opts.Command = []string{"psql"}
				opts.Args = []string{
					"-h", bind.Address,
					"-p", fmt.Sprintf("%d", bind.Port),
					database, "postgres",
				}

				// return runtime.RunAttachedStdio(ctx, res.Env, rt, runtime.DeployableSpec{
				// 	PackageRef:    &schema.PackageRef{PackageName: bind.PackageName},
				// 	Attachable:    runtime.AttachableKind_WITH_TTY,
				// 	Class:         schema.DeployableClass_ONESHOT,
				// 	Id:            ids.NewRandomBase32ID(8),
				// 	Name:          "psql",
				// 	MainContainer: opts,
				// })
				return nil
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
			Use:   "pgdump [--database <database-name>] [--out <file>]",
			Short: "Performs a dump of the contents of an existing database.",
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.StringVar(&database, "database", "", "Connect to the specified database.")
			flags.StringVar(&out, "out", "", "If set, dumps the output to the specified file.")
			_ = cobra.MarkFlagRequired(flags, "database")
		}).
		With(parseHydrationWithDeps(&res, &fncobra.ParseLocationsOpts{RequireSingle: true}, &hydrateOpts{rehydrateOnly: true})...).
		Do(func(ctx context.Context) error {
			return runPostgresCommand(ctx, database, &res, func(ctx context.Context, rt runtime.Planner, bind databaseAccessor, opts runtime.ContainerRunOpts) error {
				opts.Command = []string{"/bin/bash"}
				opts.Args = []string{}

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

				cmd := strings.NewReader(strings.Join([]string{
					"pg_dump",
					"-h", bind.Address,
					"-p", fmt.Sprintf("%d", bind.Port),
					"-U", "postgres",
					database,
				}, " "))

				return runtime.RunAttached(ctx, res.Env, rt, runtime.DeployableSpec{
					PackageRef:    &schema.PackageRef{PackageName: bind.PackageName},
					Attachable:    runtime.AttachableKind_WITH_STDIN_ONLY,
					Class:         schema.DeployableClass_ONESHOT,
					Id:            ids.NewRandomBase32ID(8),
					Name:          "pgdump",
					MainContainer: opts,
				}, runtime.TerminalIO{
					Stdin:  cmd,
					Stdout: outw,
					Stderr: os.Stderr,
				})
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
			Use:   "pgrestore [--database <database-name>] --restore <file>",
			Short: "Performs a restore of the contents of an existing backup.",
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.StringVar(&database, "database", "", "Connect to the specified database.")
			flags.StringVar(&restore, "restore", "", "The contents to be restored.")
			_ = cobra.MarkFlagRequired(flags, "database")
			_ = cobra.MarkFlagRequired(flags, "restore")
		}).
		With(parseHydrationWithDeps(&res, &fncobra.ParseLocationsOpts{RequireSingle: true}, &hydrateOpts{rehydrateOnly: true})...).
		Do(func(ctx context.Context) error {
			return runPostgresCommand(ctx, database, &res, func(ctx context.Context, rt runtime.Planner, bind databaseAccessor, opts runtime.ContainerRunOpts) error {
				opts.Command = []string{"psql"}
				opts.Args = []string{
					"-h", bind.Address,
					"-p", fmt.Sprintf("%d", bind.Port),
					database, "postgres",
				}

				f, err := os.Open(restore)
				if err != nil {
					return err
				}

				defer f.Close()

				return runtime.RunAttached(ctx, res.Env, rt, runtime.DeployableSpec{
					PackageRef:    &schema.PackageRef{PackageName: bind.PackageName},
					Attachable:    runtime.AttachableKind_WITH_STDIN_ONLY,
					Class:         schema.DeployableClass_ONESHOT,
					Id:            ids.NewRandomBase32ID(8),
					Name:          "pgrestore",
					MainContainer: opts,
				}, runtime.TerminalIO{
					Stdin:  f,
					Stdout: os.Stdout,
					Stderr: os.Stderr,
				})
			})
		})
}

type databaseAccessor struct {
	PackageName string
	Address     string
	Port        uint32
}

func determineAccessor(res *hydrateResult) (databaseAccessor, error) {
	for _, entry := range res.Rehydrated.Stack.Entry {
		// if slices.Contains(res.Focus, schema.PackageName(entry.Server.PackageName)) {
		collection, err := secrets.Collect(entry.Server)
		if err != nil {
			return databaseAccessor{}, err
		}
		log.Default().Printf("Secrets: %v\n", collection.Names)

		for _, secret := range collection.SecretsOf("namespacelabs.dev/foundation/library/oss/postgres/server") {
			log.Default().Printf("Postgres secrets: %v\n", secret)
		}
		// }
	}

	for _, entry := range res.Rehydrated.Stack.Entry {
		log.Default().Printf("entry %s", entry.Server.PackageName)
		if entry.Server.PackageName == "namespacelabs.dev/foundation/library/oss/postgres/server" {
			// for _, fragment := range res.Rehydrated.IngressFragments {
			// 	log.Default().Printf("fragment.Owner: %s\n", fragment.Owner)
			// 	// if fragment.Owner == entry.Server.PackageName {

			// 	// }
			// }

			// log.Default().Printf("Server: %v\n", entry.Server)

			// collection, err := secrets.Collect(entry.Server)
			// if err != nil {
			// 	return databaseAccessor{}, err
			// }
			// log.Default().Printf("Secrets: %v\n", collection.Names)
			log.Default().Printf("Port: %d\n", entry.Server.Service[0].Port.ContainerPort)
			log.Default().Printf("Address: %s\n", entry.Server.Service[0].Name)

			// sleep 1 sec
			time.Sleep(1 * time.Second)

			return databaseAccessor{
				PackageName: "namespacelabs.dev/foundation/library/oss/postgres/server",
				Address:     entry.Server.Service[0].Name,
				Port:        uint32(entry.Server.Service[0].Port.ContainerPort),
			}, nil
		}
	}

	return databaseAccessor{}, fnerrors.New("%s: server has no databases", res.Focus)
}

func runPostgresCommand(ctx context.Context, database string, res *hydrateResult, run func(context.Context, runtime.Planner, databaseAccessor, runtime.ContainerRunOpts) error) error {
	accessor, err := determineAccessor(res)
	if err != nil {
		return err
	}

	planner, err := runtime.PlannerFor(ctx, res.Env)
	if err != nil {
		return err
	}

	psqlImage, err := compute.GetValue(ctx,
		oci.ResolveDigest("postgres:14.3-alpine@sha256:a00af33e23643f497a42bc24d2f6f28cc67f3f48b076135c5626b2e07945ff9c",
			oci.ResolveOpts{PublicImage: true}).ImageID())
	if err != nil {
		return err
	}

	runOpts := runtime.ContainerRunOpts{
		WorkingDir: "/",
		Image:      psqlImage,
		Env: []*schema.BinaryConfig_EnvEntry{
			{
				Name:                   "PGPASSWORD",
				ExperimentalFromSecret: "namespacelabs.dev/foundation/library/oss/postgres/server:password",
			},
		},
		ReadOnlyFilesystem: true,
	}

	return run(ctx, planner, accessor, runOpts)
}
