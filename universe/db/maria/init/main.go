// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"namespacelabs.dev/foundation/universe/db/maria"

	_ "github.com/go-sql-driver/mysql"
)

const connBackoff = 3 * time.Second

var (
	passwordFile = flag.String("mariadb_password_file", "", "location of the password secret")
)

func connect(ctx context.Context, password string, address string, port uint32) (db *sql.DB, err error) {
	connString := fmt.Sprintf("root:%s@tcp(%s:%d)/", password, address, port)
	err = backoff.Retry(func() error {
		log.Printf("Connecting to MariaDB.")
		db, err = sql.Open("mysql", connString)
		if err != nil {
			log.Printf("Failed to connect to MariaDB: %v", err)
		}
		return err
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

	if err := conn.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping connection: %w", err)
	}
	log.Printf("Pinged database.")

	// SQL arguments can only be values, not identifiers.
	// As we need to use Sprintf instead, let's do some basic sanity checking (whitespaces are forbidden).
	if len(strings.Fields(db.Name)) > 1 {
		return nil, fmt.Errorf("Invalid database name: %s", db.Name)
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

	schema, err := ioutil.ReadFile(db.SchemaFile.Path)
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
		file, err := ioutil.ReadFile(path)
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
	password, err := ioutil.ReadFile(*passwordFile)
	if err != nil {
		log.Fatalf("unable to read file %s: %v", *passwordFile, err)
	}

	dbs, err := readConfigs()
	if err != nil {
		log.Fatalf("%v", err)
	}

	for _, db := range dbs {
		if err := applySchema(ctx, db, string(password)); err != nil {
			log.Fatalf("%v", err)
		}
	}
	log.Printf("mariadb init completed")
}
