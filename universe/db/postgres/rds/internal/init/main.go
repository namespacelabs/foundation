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
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	awsrds "github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/smithy-go"
	"github.com/cenkalti/backoff/v4"
	"github.com/dustin/go-humanize"
	"github.com/jackc/pgx/v4"
	"golang.org/x/sync/errgroup"
	"namespacelabs.dev/foundation/universe/db/postgres"
	"namespacelabs.dev/foundation/universe/db/postgres/rds/internal"
)

const connBackoff = 1 * time.Second

var (
	envName            = flag.String("env_name", "", "Name of current environment.")
	vpcID              = flag.String("eks_vpc_id", "", "VPC ID of the current EKS cluster.")
	awsCredentialsFile = flag.String("aws_credentials_file", "", "Path to the AWS credentials file.")
	userFile           = flag.String("postgres_user_file", "", "location of the user secret")
	passwordFile       = flag.String("postgres_password_file", "", "location of the password secret")

	engine = "postgres"

	// TODO configurable?!
	storage          = int32(100) // min GB
	class            = "db.m5d.xlarge"
	iops             = int32(3000)
	deleteProtection = true // can still be disabled and deleted by hand
	public           = false
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

func explain(err error) error {
	child := err
	for {
		log.Printf("Type: %v", reflect.TypeOf(child))
		child = errors.Unwrap(child)
		if child == nil {
			break
		}
	}
	return fmt.Errorf("%w", err)
}

func ensureSecurityGroup(ctx context.Context, ec2cli *ec2.Client, clusterId, vpcId string) (string, error) {
	name := fmt.Sprintf("%s-security-group", clusterId)
	desc := fmt.Sprintf("Security group for %s", clusterId)
	res, err := ec2cli.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   &name,
		Description: &desc,
		VpcId:       vpcID,
	})
	if err == nil {
		log.Printf("Created security group %s", name)
		return *res.GroupId, nil
	}

	// Apparently there's no nicer type for this.
	var e smithy.APIError
	if errors.As(err, &e) && e.ErrorCode() == "InvalidGroup.Duplicate" {
		log.Printf("Security group %s already exists", name)
		res, err := ec2cli.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
			GroupNames: []string{name},
		})
		if err != nil {
			return "", err
		}

		if len(res.SecurityGroups) != 1 {
			return "", fmt.Errorf("Expected one security group with name %s, got %d", name, len(res.SecurityGroups))
		}

		return *res.SecurityGroups[0].GroupId, nil
	}

	return "", fmt.Errorf("failed to create security group: %v", explain(err))
}

func prepareCluster(ctx context.Context, envName, vpcId string, rdscli *awsrds.Client, ec2cli *ec2.Client, dbGroup string, db *postgres.Database, user, password string) error {
	id := internal.ClusterIdentifier(envName, db.Name)

	groupId, err := ensureSecurityGroup(ctx, ec2cli, id, vpcId)
	if err != nil {
		return err
	}

	create := &awsrds.CreateDBClusterInput{
		DBClusterIdentifier:    &id,
		DatabaseName:           &db.Name,
		MasterUsername:         &user,
		MasterUserPassword:     &password,
		Engine:                 &engine, // Also set engine version?
		AllocatedStorage:       &storage,
		DBClusterInstanceClass: &class,
		Iops:                   &iops,
		DeletionProtection:     &deleteProtection,
		PubliclyAccessible:     &public,
		DBSubnetGroupName:      &dbGroup,
		VpcSecurityGroupIds:    []string{groupId},
	}

	if _, err := rdscli.CreateDBCluster(ctx, create); err != nil {
		var e *rdstypes.DBClusterAlreadyExistsFault
		if errors.As(err, &e) {
			log.Printf("RDS DB cluster %s already exists", id)
			// TODO ModifyDBCluster?
		} else {
			return fmt.Errorf("failed to create database cluster: %v", explain(err))
		}
	} else {
		log.Printf("Creating RDS DB cluster %s", id)
	}

	resp, err := rdscli.DescribeDBClusters(ctx, &awsrds.DescribeDBClustersInput{
		DBClusterIdentifier: &id,
	})
	if err != nil {
		return err
	}

	if len(resp.DBClusters) != 1 {
		return fmt.Errorf("Expected one cluster with identifier %s, got %d", id, len(resp.DBClusters))
	}

	db.HostedAt = &postgres.Endpoint{
		Address: *resp.DBClusters[0].Endpoint,
		Port:    uint32(*resp.DBClusters[0].Port),
	}

	// Wait for cluster to be ready
	// TODO tidy up
wait:
	for {
		// watch would be nice
		time.Sleep(connBackoff)

		resp, err := rdscli.DescribeDBClusterEndpoints(ctx, &awsrds.DescribeDBClusterEndpointsInput{
			DBClusterIdentifier: &id,
		})
		if err != nil {
			return err
		}

		if len(resp.DBClusterEndpoints) == 0 {
			// keep waiting
			continue
		}

		for _, endpoint := range resp.DBClusterEndpoints {
			log.Printf("Endpoint %s has status %s", *endpoint.DBClusterEndpointIdentifier, *endpoint.Status)
			if *endpoint.Status != "available" {
				continue wait
			}
		}

		return prepareDatabase(ctx, db, user, password)
	}
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
		log.Fatalf("Required flag --aws_credentials_file is not set.")
	}

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedCredentialsFiles([]string{*awsCredentialsFile}))
	if err != nil {
		log.Fatalf("Failed to load aws config: %v", err)
	}

	// TODO do we really want to reuse the same VPC, or rather create a new one + peering?
	// https://aws.amazon.com/about-aws/whats-new/2021/05/amazon-vpc-announces-pricing-change-for-vpc-peering/
	log.Printf("EKS VPC is %q", *vpcID)

	rdscli := awsrds.NewFromConfig(awsCfg)
	ec2cli := ec2.NewFromConfig(awsCfg)

	if *envName == "" {
		log.Fatalf("Required flag --env_name is not set.")
	}

	dbGroup, err := createDBSubnetGroup(ctx, *envName, rdscli, ec2cli, *vpcID)
	if err != nil {
		log.Fatalf("unable to create DB subnet group: %v", err)
	}

	dbs, err := readConfigs()
	if err != nil {
		log.Fatalf("%v", err)
	}

	eg, ctx := errgroup.WithContext(ctx)
	for _, db := range dbs {
		db := db // Close db
		eg.Go(func() error {
			return prepareCluster(ctx, *envName, *vpcID, rdscli, ec2cli, dbGroup, db, user, password)
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

func createDBSubnetGroup(ctx context.Context, envName string, rdscli *awsrds.Client, ec2cli *ec2.Client, vpcId string) (string, error) {
	idFilter := "vpc-id"
	resp, err := ec2cli.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []ec2types.Filter{{
			Name:   &idFilter,
			Values: []string{vpcId},
		}},
	})
	if err != nil {
		return "", fmt.Errorf("Unable to list subnets for VPC %s: %v", vpcId, explain(err))
	}

	if len(resp.Subnets) == 0 {
		return "", fmt.Errorf("Found no subnets for VPC %s", vpcId)
	}

	var subnetIds []string
	for _, subnet := range resp.Subnets {
		subnetIds = append(subnetIds, *subnet.SubnetId)
	}

	name := fmt.Sprintf("ns-%s-db-subnet", envName)
	desc := fmt.Sprintf("Namespace DB Subnet group for RDS deployments in %s environment.", envName)
	if _, err := rdscli.CreateDBSubnetGroup(ctx, &awsrds.CreateDBSubnetGroupInput{
		DBSubnetGroupName:        &name,
		DBSubnetGroupDescription: &desc,
		SubnetIds:                subnetIds, // TODO Should we create our own?
	}); err != nil {
		var e *rdstypes.DBSubnetGroupAlreadyExistsFault
		if errors.As(err, &e) {
			log.Printf("RDS DB subnet group %s already exists", name)
			return name, nil
		} else {
			return "", fmt.Errorf("failed to create db subnet group: %v", explain(err))
		}
	}

	log.Printf("Created DB subnet group %q", name)
	return name, nil
}
