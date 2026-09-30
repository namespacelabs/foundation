// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/jackc/pgx/v5"
	"namespacelabs.dev/foundation/framework/resources"
	"namespacelabs.dev/foundation/framework/resources/provider"
	cockroachclass "namespacelabs.dev/foundation/library/database/cockroach"
	"namespacelabs.dev/foundation/library/oss/cockroach"
	"namespacelabs.dev/foundation/library/oss/postgres"
)

const (
	providerPkg = "namespacelabs.dev/foundation/library/oss/cockroach"
	user        = "postgres"
)

func main() {
	ctx, p := provider.MustPrepare[*cockroach.ClusterIntent]()

	endpoint, err := resources.LookupServerEndpoint(p.Resources, fmt.Sprintf("%s:server", providerPkg), "postgres")
	if err != nil {
		log.Fatalf("failed to get cockroach server endpoint: %v", err)
	}

	password, err := resources.ReadSecret(p.Resources, fmt.Sprintf("%s:password", providerPkg))
	if err != nil {
		log.Fatalf("failed to read cockroach password: %v", err)
	}

	instance := &cockroachclass.ClusterInstance{
		Address:  endpoint,
		User:     user,
		Password: string(password),
	}

	if err := waitForAdmin(ctx, instance); err != nil {
		log.Fatalf("failed to initialize cockroach cluster: %v", err)
	}

	p.EmitResult(instance)
}

func waitForAdmin(ctx context.Context, cluster *cockroachclass.ClusterInstance) error {
	cfg, err := pgx.ParseConfig(postgres.ConnectionUri(cluster, "postgres"))
	if err != nil {
		return err
	}
	cfg.ConnectTimeout = time.Second

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// The image creates the user before granting admin. Authentication alone can
	// succeed while dependent database providers would still get permission denied.
	return backoff.Retry(func() error {
		conn, err := pgx.ConnectConfig(ctx, cfg)
		if err != nil {
			return err
		}
		defer func() {
			if err := conn.Close(ctx); err != nil {
				log.Printf("unable to close cockroach connection: %v", err)
			}
		}()

		var admin bool
		if err := conn.QueryRow(ctx, "SELECT pg_has_role(current_user, 'admin', 'MEMBER')").Scan(&admin); err != nil {
			return err
		}
		if !admin {
			return fmt.Errorf("user %q is waiting for the admin grant", cluster.User)
		}
		return nil
	}, backoff.WithContext(backoff.NewConstantBackOff(time.Second), ctx))
}
