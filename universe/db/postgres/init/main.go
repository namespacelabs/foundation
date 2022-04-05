// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"namespacelabs.dev/foundation/universe/db/postgres"
)

const connBackoff = 3 * time.Second

var (
	userFile     = flag.String("postgres_user_file", "", "location of the user secret")
	passwordFile = flag.String("postgres_password_file", "", "location of the password secret")
)

func existsDb(ctx context.Context, conn *pgxpool.Pool, dbName string) (bool, error) {
	rows, err := conn.Query(ctx, "SELECT FROM pg_database WHERE datname = $1;", dbName)
	if err != nil {
		return false, fmt.Errorf("failed to check for database %s: %w", dbName, err)
	}
	defer rows.Close()

	return rows.Next(), nil
}

func connect(ctx context.Context, user string, password string, address string, port uint32, db string) (conn *pgxpool.Pool, err error) {
	connString := fmt.Sprintf("postgres://%s:%s@%s:%d/%s", user, password, address, port, db)
	err = backoff.Retry(func() error {
		log.Printf("Connecting to postgres.")
		conn, err = pgxpool.Connect(ctx, connString)
		if err != nil {
			log.Printf("Failed to connect to postgres: %v", err)
		}
		return err
	}, backoff.WithContext(backoff.NewConstantBackOff(connBackoff), ctx))

	if err != nil {
		return nil, fmt.Errorf("unable to establish postgres connection: %w", err)
	}

	return conn, nil
}

func ensureDb(ctx context.Context, db *postgres.Database, user string, password string) error {
	// Postgres needs a db to connect to so we pin one that is guaranteed to exist.
	conn, err := connect(ctx, user, password, db.HostedAt.Address, db.HostedAt.Port, "postgres")
	if err != nil {
		return err
	}
	defer conn.Close()

	// Postgres does not support CREATE DATABASE IF NOT EXISTS
	log.Printf("Querying for existing databases.")
	exists, err := existsDb(ctx, conn, db.Name)
	if err != nil {
		return err
	}

	if exists {
		log.Printf("Database `%s` already exists.", db.Name)
		return nil
	}

	// SQL arguments can only be values, not identifiers.
	// https://www.postgresql.org/docs/9.5/xfunc-sql.html
	// As we need to use Sprintf instead, let's do some basic sanity checking (whitespaces are forbidden).
	if len(strings.Fields(db.Name)) > 1 {
		return fmt.Errorf("Invalid database name: %s", db.Name)
	}
	_, err = conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s;", db.Name))
	if err != nil {
		return fmt.Errorf("failed to create database `%s`: %w", db.Name, err)
	}

	log.Printf("Created database `%s`.", db.Name)
	return nil
}

func applySchema(ctx context.Context, db *postgres.Database, user string, password string) error {
	conn, err := connect(ctx, user, password, db.HostedAt.Address, db.HostedAt.Port, db.Name)
	if err != nil {
		return err
	}
	defer conn.Close()

	schema, err := ioutil.ReadFile(db.SchemaFile.Path)
	if err != nil {
		return fmt.Errorf("unable to read file %s: %v", db.SchemaFile.Path, err)
	}

	log.Printf("Applying schema %s.", db.SchemaFile.Path)
	_, err = conn.Exec(ctx, string(schema))
	if err != nil {
		return fmt.Errorf("unable to execute schema %s: %v", db.SchemaFile.Path, err)
	}
	return nil
}

func readConfigs() ([]*postgres.Database, error) {
	dbs := []*postgres.Database{}

	for _, path := range flag.Args() {
		file, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("unable to read file %s: %v", path, err)
		}

		db := &postgres.Database{}
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

	dbs, err := readConfigs()
	if err != nil {
		log.Fatalf("%v", err)
	}

	for _, db := range dbs {
		if err := ensureDb(ctx, db, user, string(password)); err != nil {
			log.Fatalf("%v", err)
		}
		if err := applySchema(ctx, db, user, string(password)); err != nil {
			log.Fatalf("%v", err)
		}
	}
	log.Printf("postgres init completed")
}
