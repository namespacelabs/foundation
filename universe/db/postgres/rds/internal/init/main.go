// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	awsrds "github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/cenkalti/backoff/v4"
	"github.com/dustin/go-humanize"
	"github.com/jackc/pgx/v4"
	"golang.org/x/sync/errgroup"
	"namespacelabs.dev/foundation/universe/db/postgres"
	"namespacelabs.dev/foundation/universe/db/postgres/rds/internal"
)

const connBackoff = 1 * time.Second

var (
	awsCredentialsFile = flag.String("aws_credentials_file", "", "Path to the AWS credentials file.")
	userFile           = flag.String("postgres_user_file", "", "location of the user secret")
	passwordFile       = flag.String("postgres_password_file", "", "location of the password secret")

	engine = "postgres"

	// TODO configurable?
	storage = int32(100) // min GB
	class   = "db.m5d.xlarge"
	iops    = int32(3000)
)

// TODO dedup
func existsDb(ctx context.Context, conn *pgx.Conn, dbName string) (bool, error) {
	rows, err := conn.Query(ctx, "SELECT FROM pg_database WHERE datname = $1;", dbName)
	if err != nil {
		return false, fmt.Errorf("failed to check for database %s: %w", dbName, err)
	}
	defer rows.Close()

	return rows.Next(), nil
}

// TODO dedup
func connect(ctx context.Context, user string, password string, address string, port uint32, db string) (conn *pgx.Conn, err error) {
	connString := fmt.Sprintf("postgres://%s:%s@%s:%d/%s", user, password, address, port, db)
	count := 0
	err = backoff.Retry(func() error {
		addrPort := fmt.Sprintf("%s:%d", address, port)

		// Use a more aggressive connect to determine whether the server already
		// has an open serving port. If it does, we then defer to pgx.Connect to
		// take as much time as it needs.
		rawConn, err := net.DialTimeout("tcp", addrPort, 3*connBackoff)
		if err != nil {
			log.Printf("Failed to tcp dial %s: %v", addrPort, err)
			return err
		}

		rawConn.Close()

		count++
		log.Printf("Connecting to postgres (%s try), address is `%s:%d`.", humanize.Ordinal(count), address, port)
		conn, err = pgx.Connect(ctx, connString)
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

// TODO dedup
func ensureDb(ctx context.Context, conn *pgx.Conn, db *postgres.Database) error {
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
		return fmt.Errorf("invalid database name: %s", db.Name)
	}

	if _, err := conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s;", db.Name)); err != nil {
		return fmt.Errorf("failed to create database `%s`: %w", db.Name, err)
	}

	log.Printf("Created database `%s`.", db.Name)
	return nil
}

// TODO dedup
func applySchema(ctx context.Context, conn *pgx.Conn, db *postgres.Database) error {
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

// TODO dedup
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

// TODO dedup
func prepareDatabase(ctx context.Context, db *postgres.Database, user, password string) error {
	// Postgres needs a db to connect to so we pin one that is guaranteed to exist.
	postgresDB, err := connect(ctx, user, password, db.HostedAt.Address, db.HostedAt.Port, "postgres")
	if err != nil {
		return err
	}
	defer postgresDB.Close(ctx)

	if err := ensureDb(ctx, postgresDB, db); err != nil {
		return err
	}

	conn, err := connect(ctx, user, password, db.HostedAt.Address, db.HostedAt.Port, db.Name)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	return applySchema(ctx, conn, db)
}

func prepareCluster(ctx context.Context, rdscli *awsrds.Client, db *postgres.Database, user, password string) error {
	id := internal.ClusterIdentifier(db.Name)
	create := &awsrds.CreateDBClusterInput{
		DBClusterIdentifier:    &id,
		DatabaseName:           &db.Name,
		MasterUsername:         &user,
		MasterUserPassword:     &password,
		Engine:                 &engine, // Also set engine version?
		AllocatedStorage:       &storage,
		DBClusterInstanceClass: &class,
		Iops:                   &iops,
	}

	if _, err := rdscli.CreateDBCluster(ctx, create); err != nil {
		var e *types.DBClusterAlreadyExistsFault
		if errors.As(err, &e) {
			// TODO update?
		} else {
			return fmt.Errorf("failed to create database cluster: %v (type: %v)", err, reflect.TypeOf(err))
		}
	}

	desc, err := rdscli.DescribeDBClusters(ctx, &awsrds.DescribeDBClustersInput{
		DBClusterIdentifier: &id,
	})
	if err != nil {
		return err
	}

	if len(desc.DBClusters) != 1 {
		return fmt.Errorf("Expected one cluster with identifier %s, got %d", id, len(desc.DBClusters))
	}

	db.HostedAt = &postgres.Endpoint{
		Address: *desc.DBClusters[0].Endpoint,
		Port:    uint32(*desc.DBClusters[0].Port),
	}

	return prepareDatabase(ctx, db, user, password)
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	flag.Parse()

	log.Printf("postgres init begins")
	ctx := context.Background()

	user, err := readUser()
	if err != nil {
		log.Fatalf("%v", err)
	}

	password, err := readPassword()
	if err != nil {
		log.Fatalf("%v", err)
	}

	if *awsCredentialsFile == "" {
		log.Fatalf("aws_credentials_file must be set")
	}

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedCredentialsFiles([]string{*awsCredentialsFile}))
	if err != nil {
		log.Fatalf("Failed to load aws config: %s", err)
	}
	rdscli := awsrds.NewFromConfig(awsCfg)

	dbs, err := readConfigs()
	if err != nil {
		log.Fatalf("%v", err)
	}

	eg, ctx := errgroup.WithContext(ctx)
	for _, db := range dbs {
		db := db // Close db
		eg.Go(func() error {
			return prepareCluster(ctx, rdscli, db, user, password)
		})
	}

	if err := eg.Wait(); err != nil {
		log.Fatal(err)
	}

	log.Printf("postgres init completed")
}

func readUser() (string, error) {
	if *userFile == "" {
		return "postgres", nil
	}

	user, err := ioutil.ReadFile(*userFile)
	if err != nil {
		return "", fmt.Errorf("unable to read file %s: %v", *userFile, err)
	}

	return string(user), nil
}

func readPassword() (string, error) {
	pw, err := ioutil.ReadFile(*passwordFile)
	if err != nil {
		return "", fmt.Errorf("unable to read file %s: %v", *passwordFile, err)
	}

	return string(pw), nil
}
