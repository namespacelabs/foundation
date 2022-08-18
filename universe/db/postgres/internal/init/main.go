// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"flag"
	"io/ioutil"
	"log"

	"golang.org/x/sync/errgroup"
	"namespacelabs.dev/foundation/universe/db/postgres/internal/initcommon"
)

var (
	userFile     = flag.String("postgres_user_file", "", "location of the user secret")
	passwordFile = flag.String("postgres_password_file", "", "location of the password secret") // should be per DB
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	flag.Parse()
	log.Printf("postgres init begins")
	ctx := context.Background()

	// TODO: creds should be definable per db instance #217
	user := "postgres"
	if *userFile != "" {
		bytes, err := ioutil.ReadFile(*userFile)
		if err != nil {
			log.Fatalf("unable to read file %s: %v", *userFile, err)
		}
		user = string(bytes)
	}

	password, err := ioutil.ReadFile(*passwordFile)
	if err != nil {
		log.Fatalf("unable to read file %s: %v", *passwordFile, err)
	}

	dbs, err := initcommon.ReadConfigs()
	if err != nil {
		log.Fatalf("%v", err)
	}

	eg, ctx := errgroup.WithContext(ctx)
	for _, db := range dbs {
		db := db // Close db
		eg.Go(func() error {
			return initcommon.PrepareDatabase(ctx, db, user, string(password))
		})
	}

	if err := eg.Wait(); err != nil {
		log.Fatal(err)
	}

	log.Printf("postgres init completed")
}
