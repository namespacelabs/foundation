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
	"os"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"golang.org/x/xerrors"
	"namespacelabs.dev/foundation/universe/db/postgres"
)

const connBackoff = 3 * time.Second

var (
	userFile     = flag.String("postgres_user_file", "", "location of the user secret")
	passwordFile = flag.String("postgres_password_file", "", "location of the password secret")
)

func logf(message string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, "%s : %s\n", time.Now().String(), fmt.Sprintf(message, args...))
}

func existsDb(ctx context.Context, conn *pgxpool.Pool, dbName string) (bool, error) {
	rows, err := conn.Query(ctx, "SELECT FROM pg_database WHERE datname = $1;", dbName)
	if err != nil {
		return false, xerrors.Errorf("failed to check for database %s: %w", dbName, err)
	}
	defer rows.Close()

	return rows.Next(), nil
}

func connect(ctx context.Context, connString string) (conn *pgxpool.Pool, err error) {
	err = backoff.Retry(func() error {
		logf("Connecting to postgres.")
		conn, err = pgxpool.Connect(ctx, connString)
		if err != nil {
			logf("Failed to connect to postgres: %v", err)
		}
		return err
	}, backoff.WithContext(backoff.NewConstantBackOff(connBackoff), ctx))

	if err != nil {
		return nil, xerrors.Errorf("unable to establish postgres connection: %w", err)
	}

	return conn, nil
}

func ensureDb(ctx context.Context, db *postgres.Database, user string, password string) error {
	conn, err := connect(ctx, fmt.Sprintf("postgres://%s:%s@%s:%d", user, password, db.HostedAt.Address, db.HostedAt.Port))
	if err != nil {
		return err
	}
	defer conn.Close()

	// Postgres does not support CREATE DATABASE IF NOT EXISTS
	logf("Querying for existing databases.")
	exists, err := existsDb(ctx, conn, db.Name)
	if err != nil {
		return err
	}

	if exists {
		logf("Database %s already exists.", db.Name)
		return nil
	}

	// SQL arguments can only be values, not identifiers.
	// https://www.postgresql.org/docs/9.5/xfunc-sql.html
	// As we need to use Sprintf instead, let's do some basic sanity checking (whitespaces are forbidden).
	if len(strings.Fields(db.Name)) > 1 {
		return xerrors.Errorf("Invalid database name: %s", db.Name)
	}
	_, err = conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s;", db.Name))
	if err != nil {
		return xerrors.Errorf("failed to create database %s: %w", db.Name, err)
	}

	logf("Created database %s.", db.Name)
	return nil
}

func applySchema(ctx context.Context, db *postgres.Database, user string, password string) error {
	conn, err := connect(ctx, fmt.Sprintf("postgres://%s:%s@%s:%d/%s", user, password, db.HostedAt.Address, db.HostedAt.Port, db.Name))
	if err != nil {
		return err
	}
	defer conn.Close()

	schema, err := ioutil.ReadFile(db.SchemaFile.Path)
	if err != nil {
		return xerrors.Errorf("unable to read file %s: %v", db.SchemaFile.Path, err)
	}

	logf("Applying schema %s.", db.SchemaFile.Path)
	_, err = conn.Exec(ctx, string(schema))
	if err != nil {
		return xerrors.Errorf("unable to execute schema %s: %v", db.SchemaFile.Path, err)
	}
	return nil
}

func readConfigs() ([]*postgres.Database, error) {
	dbs := []*postgres.Database{}

	for _, path := range flag.Args() {
		file, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, xerrors.Errorf("unable to read file %s: %v", path, err)
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
	flag.Parse()
	logf("postgres init begins")
	ctx := context.Background()

	// TODO: creds should be definable per db instance #217
	user, err := ioutil.ReadFile(*userFile)
	if err != nil {
		log.Fatalf("unable to read file %s: %v", *userFile, err)
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
		if err := ensureDb(ctx, db, string(user), string(password)); err != nil {
			log.Fatalf("%v", err)
		}
		if err := applySchema(ctx, db, string(user), string(password)); err != nil {
			log.Fatalf("%v", err)
		}
	}
	logf("postgres init completed")
}