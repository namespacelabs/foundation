// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"namespacelabs.dev/foundation/framework/resources/provider"
	postgresclass "namespacelabs.dev/foundation/library/database/postgres"
	"namespacelabs.dev/foundation/library/oss/postgres"
	"namespacelabs.dev/foundation/library/oss/postgres/prepare/database/helpers"
	universepg "namespacelabs.dev/foundation/universe/db/postgres"
)

const (
	providerPkg     = "namespacelabs.dev/foundation/library/oss/postgres"
	connIdleTimeout = 15 * time.Minute

	caCertPath = "/tmp/ca.pem"
)

func main() {
	ctx, p := provider.MustPrepare[*postgres.DatabaseIntent]()

	if err := run(ctx, p); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, p *provider.Provider[*postgres.DatabaseIntent]) error {
	cluster := &postgresclass.ClusterInstance{}
	if err := p.Resources.Unmarshal(fmt.Sprintf("%s:cluster", providerPkg), cluster); err != nil {
		return fmt.Errorf("unable to read required resource \"cluster\": %w", err)
	}

	// TODO inject file as secret ref and propagate secret ref to server, too.
	if cluster.CaCert != "" {
		if err := os.WriteFile(caCertPath, []byte(cluster.CaCert), 0644); err != nil {
			return fmt.Errorf("failed to write %q: %w", caCertPath, err)
		}

		if err := os.Setenv("PGSSLROOTCERT", caCertPath); err != nil {
			return fmt.Errorf("failed to set PGSSLROOTCERT: %w", err)
		}

	}

	exists, err := helpers.EnsureDatabase(ctx, cluster, p.Intent.Name, "")
	if err != nil {
		return fmt.Errorf("unable to create database %q: %w", p.Intent.Name, err)
	}

	instance := &postgresclass.DatabaseInstance{
		ConnectionUri:  postgres.ConnectionUri(cluster, p.Intent.Name),
		Name:           p.Intent.Name,
		User:           postgres.UserOrDefault(cluster.User),
		Password:       cluster.Password,
		ClusterAddress: cluster.Address,
		ClusterHost:    cluster.Host,
		ClusterPort:    cluster.Port,
		SslMode:        cluster.SslMode,
		EnableTracing:  p.Intent.EnableTracing,
	}

	if !exists || !p.Intent.SkipSchemaInitializationIfExists {
		client := fmt.Sprintf("provider:%s", p.Intent.Name)
		db, err := universepg.NewDatabaseFromConnectionUriWithOverrides(ctx, instance, instance.ConnectionUri, nil, client, &universepg.ConfigOverrides{
			MaxConnIdleTime: connIdleTimeout,
		})
		if err != nil {
			return fmt.Errorf("unable to open connection: %w", err)
		}
		defer func() {
			if err := db.Close(); err != nil {
				log.Printf("unable to close database connection: %v", err)
			}
		}()

		if err := helpers.ApplyWithHelpers(ctx, p.Intent, db); err != nil {
			return err
		}
	}

	p.EmitResult(instance)
	return nil
}
