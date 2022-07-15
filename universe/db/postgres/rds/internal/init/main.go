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
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	awsrds "github.com/aws/aws-sdk-go-v2/service/rds"
	"golang.org/x/sync/errgroup"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/universe/db/postgres"
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
func prepareDatabase(ctx context.Context, rdscli *awsrds.Client, db *postgres.Database, user, password string) error {
	input := &awsrds.CreateDBClusterInput{
		Engine:                 &engine, // Also set engine version?
		AllocatedStorage:       &storage,
		DBClusterInstanceClass: &class,
		Iops:                   &iops,
		MasterUsername:         &user,
		MasterUserPassword:     &password,
		DatabaseName:           &db.Name,
	}

	out, err := rdscli.CreateDBCluster(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to create database cluster: %v", err)
	}

	// Debug
	serialized, err := json.MarshalIndent(out, "", " ")
	if err != nil {
		return fmt.Errorf("failed to mashal response: %v", err)
	}
	fmt.Fprintf(console.Stdout(ctx), "rdscli.CreateDBCluster:\n%s\n", string(serialized))

	return nil
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
			return prepareDatabase(ctx, rdscli, db, user, password)
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
