// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/dustin/go-humanize"
	"golang.org/x/sync/errgroup"
	"namespacelabs.dev/foundation/universe/db/maria"

	_ "github.com/go-sql-driver/mysql"
)

const connBackoff = 1 * time.Second

var (
	passwordFile = flag.String("mariadb_password_file", "", "location of the password secret")
)

func connect(ctx context.Context, password string, address string, port uint32) (db *sql.DB, err error) {
	connString := fmt.Sprintf("root:%s@tcp(%s:%d)/", password, address, port)
	count := 0
	err = backoff.Retry(func() error {
		addrPort := fmt.Sprintf("%s:%d", address, port)

		// Use a more aggressive connect to determine whether the server already
		// has an open serving port. If it does, we then defer to sql.Open to
		// take as much time as it needs.
		rawConn, err := net.DialTimeout("tcp", addrPort, 3*connBackoff)
		if err != nil {
			log.Printf("Failed to tcp dial %s: %v", addrPort, err)
			return err
		}

		rawConn.Close()

		count++
		log.Printf("Attempting to connect to MariaDB (%s try).", humanize.Ordinal(count))
		db, err = sql.Open("mysql", connString)
		if err != nil {
			log.Printf("Failed to connect to MariaDB: %v", err)
			return err
		}
		if err := db.PingContext(ctx); err != nil {
			log.Printf("Failed to ping connection: %v", err)
			return err
		}
		return nil
	}, backoff.WithContext(backoff.NewConstantBackOff(connBackoff), ctx))

	if err != nil {
		return nil, fmt.Errorf("unable to establish MariaDB connection: %w", err)
	}

	return db, nil
}

func ensureDb(ctx context.Context, db *maria.Database, password string) (*sql.DB, error) {
	conn, err := connect(ctx, password, db.HostedAt.Address, db.HostedAt.Port)
	if err != nil {
		return nil, err
	}

	// SQL arguments can only be values, not identifiers.
	// As we need to use Sprintf instead, let's do some basic sanity checking (whitespaces are forbidden).
	if len(strings.Fields(db.Name)) > 1 {
		return nil, fmt.Errorf("invalid database name: %s", db.Name)
	}

	_, err = conn.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", db.Name))
	if err != nil {
		return nil, fmt.Errorf("failed to create database `%s`: %w", db.Name, err)
	}

	log.Printf("Created database `%s`.", db.Name)

	return conn, nil
}

func applySchema(ctx context.Context, db *maria.Database, password string) error {
	conn, err := ensureDb(ctx, db, string(password))
	if err != nil {
		return err
	}
	defer conn.Close()

	schema, err := os.ReadFile(db.SchemaFile.Path)
	if err != nil {
		return fmt.Errorf("unable to read file %s: %v", db.SchemaFile.Path, err)
	}

	log.Printf("Applying schema %s.", db.SchemaFile.Path)
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("unable to create transaction: %v", err)
	}
	if _, err = tx.ExecContext(ctx, fmt.Sprintf("USE %s;", db.Name)); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, string(schema)); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("unable to execute schema %s: %v", db.SchemaFile.Path, err)
	}
	return nil
}

func readConfigs() ([]*maria.Database, error) {
	dbs := []*maria.Database{}

	for _, path := range flag.Args() {
		file, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("unable to read file %s: %v", path, err)
		}

		db := &maria.Database{}
		if err := json.Unmarshal(file, db); err != nil {
			return nil, err
		}
		dbs = append(dbs, db)
	}

	return dbs, nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	flag.Parse()
	log.Printf("mariadb init begins")
	ctx := context.Background()

	// TODO: creds should be definable per db instance #217
	password, err := os.ReadFile(*passwordFile)
	if err != nil {
		log.Fatalf("unable to read file %s: %v", *passwordFile, err)
	}

	dbs, err := readConfigs()
	if err != nil {
		log.Fatalf("%v", err)
	}

	eg, ctx := errgroup.WithContext(ctx)
	for _, db := range dbs {
		db := db // Close db
		eg.Go(func() error {
			return applySchema(ctx, db, string(password))
		})
	}

	if err := eg.Wait(); err != nil {
		log.Fatal(err)
	}

	log.Printf("mariadb init completed")
}
