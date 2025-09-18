// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"namespacelabs.dev/foundation/framework/resources/provider"
	cockroachclass "namespacelabs.dev/foundation/library/database/cockroach"
	"namespacelabs.dev/foundation/library/oss/cockroach"
	"namespacelabs.dev/foundation/library/oss/postgres"
	"namespacelabs.dev/foundation/library/oss/postgres/prepare/database/helpers"
	universepg "namespacelabs.dev/foundation/universe/db/postgres"
)

const (
	providerPkg     = "namespacelabs.dev/foundation/library/oss/cockroach"
	connIdleTimeout = 15 * time.Minute
	caCertPath      = "/tmp/ca.pem"
)

func main() {
	ctx, p := provider.MustPrepare[*cockroach.DatabaseIntent]()

	if err := run(ctx, p); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, p *provider.Provider[*cockroach.DatabaseIntent]) error {
	cluster := &cockroachclass.ClusterInstance{}
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

	var sb strings.Builder
	if len(p.Intent.Regions) > 0 {
		sb.WriteString(fmt.Sprintf("PRIMARY REGION %q REGIONS ", p.Intent.Regions[0]))
		for i, region := range p.Intent.Regions {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%q", region))
		}

		if p.Intent.SurvivalGoal != "" {
			sb.WriteString(fmt.Sprintf(" SURVIVE %s FAILURE", p.Intent.SurvivalGoal))
		}
	}

	exists, err := helpers.EnsureDatabase(ctx, cluster, p.Intent.Name, sb.String())
	if err != nil {
		return fmt.Errorf("unable to create database %q: %w", p.Intent.Name, err)
	}

	instance := &cockroachclass.DatabaseInstance{
		ConnectionUri:  postgres.ConnectionUri(cluster, p.Intent.Name),
		Name:           p.Intent.Name,
		User:           postgres.UserOrDefault(cluster.User),
		Password:       cluster.Password,
		ClusterAddress: cluster.Address,
		ClusterHost:    cluster.Host,
		ClusterPort:    cluster.Port,
		SslMode:        cluster.SslMode,
		EnableTracing:  p.Intent.EnableTracing,
		SurvivalGoal:   p.Intent.SurvivalGoal,
		Regions:        p.Intent.Regions,
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

		for _, oneSchema := range p.Intent.Schema {
			if err := applyWithRetry(ctx, db, string(oneSchema.Contents)); err != nil {
				return fmt.Errorf("unable to apply schema %q: %w", oneSchema.Path, err)
			}
		}
	}

	p.EmitResult(instance)
	return nil
}

func applyWithRetry(ctx context.Context, db *universepg.DB, sql string) error {
	return backoff.Retry(func() error {
		_, err := db.Exec(ctx, sql)

		if !universepg.ErrorIsRetryable(err) {
			return backoff.Permanent(err)
		}

		return err
	}, helpers.BackOff{
		Interval: 10 * time.Second,
		// Leave more time for migrations to run since schema changes are relatively slow
		Deadline: time.Now().Add(5 * time.Minute),
		Jitter:   5 * time.Second,
	})
}
