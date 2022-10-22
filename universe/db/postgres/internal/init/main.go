// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"flag"
	"log"

	"golang.org/x/sync/errgroup"
	"namespacelabs.dev/foundation/universe/db/postgres/internal/initcommon"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	flag.Parse()
	log.Printf("postgres init begins")
	ctx := context.Background()

	dbs, err := initcommon.ReadConfigs()
	if err != nil {
		log.Fatalf("%v", err)
	}

	eg, ctx := errgroup.WithContext(ctx)
	for _, db := range dbs {
		db := db // Close db
		eg.Go(func() error {
			return initcommon.PrepareDatabase(ctx, db)
		})
	}

	if err := eg.Wait(); err != nil {
		log.Fatal(err)
	}

	log.Printf("postgres init completed")
}
