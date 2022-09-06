// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	awsrds "github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/smithy-go"
	"golang.org/x/sync/errgroup"
	"namespacelabs.dev/foundation/universe/db/postgres"
	"namespacelabs.dev/foundation/universe/db/postgres/internal/initcommon"
	"namespacelabs.dev/foundation/universe/db/postgres/rds/internal"
)

const connBackoff = 1 * time.Second

var (
	envName            = flag.String("env_name", "", "Name of current environment.")
	vpcID              = flag.String("eks_vpc_id", "", "VPC ID of the current EKS cluster.")
	awsCredentialsFile = flag.String("aws_credentials_file", "", "Path to the AWS credentials file.")

	ipRange = "0.0.0.0/0" // TODO lock down

	// TODO configurable?!
	storage       = int32(100)     // min GB
	class         = "db.m5d.large" // https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/Concepts.DBInstanceClass.html
	iops          = 1000           // min iops
	engineVersion = "13.4"
)

func ensureSecurityGroup(ctx context.Context, ec2cli *ec2.Client, clusterId, vpcId string) (string, error) {
	name := fmt.Sprintf("%s-security-group", clusterId)
	desc := fmt.Sprintf("Security group for %s", clusterId)
	res, err := ec2cli.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(name),
		Description: aws.String(desc),
		VpcId:       aws.String(vpcId),
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
			Filters: []ec2types.Filter{
				{Name: aws.String("group-name"), Values: []string{name}},
				{Name: aws.String("vpc-id"), Values: []string{vpcId}},
			},
		})
		if err != nil {
			return "", err
		}

		if len(res.SecurityGroups) != 1 {
			return "", fmt.Errorf("expected one security group with name %s, got %d", name, len(res.SecurityGroups))
		}

		return *res.SecurityGroups[0].GroupId, nil
	}

	return "", fmt.Errorf("failed to create security group: %v", err)
}

func prepareCluster(ctx context.Context, envName, vpcId string, rdscli *awsrds.Client, ec2cli *ec2.Client, dbGroup string, db *postgres.Database) error {
	id := internal.ClusterIdentifier(envName, db.Name)

	groupId, err := ensureSecurityGroup(ctx, ec2cli, id, vpcId)
	if err != nil {
		return err
	}

	create := &awsrds.CreateDBClusterInput{
		DBClusterIdentifier:    aws.String(id),
		DatabaseName:           aws.String(db.Name),
		MasterUsername:         aws.String(db.Credentials.User.Value),
		MasterUserPassword:     aws.String(db.Credentials.Password.Value),
		Engine:                 aws.String("postgres"),
		EngineVersion:          aws.String(engineVersion),
		AllocatedStorage:       aws.Int32(int32(storage)),
		DBClusterInstanceClass: aws.String(class),
		Iops:                   aws.Int32(int32(iops)),
		DeletionProtection:     aws.Bool(true), // can still be disabled and deleted by hand
		PubliclyAccessible:     aws.Bool(false),
		DBSubnetGroupName:      aws.String(dbGroup),
		VpcSecurityGroupIds:    []string{groupId},
	}

	if _, err := rdscli.CreateDBCluster(ctx, create); err != nil {
		var e *rdstypes.DBClusterAlreadyExistsFault
		if errors.As(err, &e) {
			log.Printf("RDS DB cluster %s already exists", id)
			// TODO ModifyDBCluster?
		} else {
			return fmt.Errorf("failed to create database cluster: %v", err)
		}
	} else {
		log.Printf("Creating RDS DB cluster %s", id)
	}

	resp, err := rdscli.DescribeDBClusters(ctx, &awsrds.DescribeDBClustersInput{
		DBClusterIdentifier: aws.String(id),
	})
	if err != nil {
		return err
	}

	if len(resp.DBClusters) != 1 {
		return fmt.Errorf("expected one cluster with identifier %s, got %d", id, len(resp.DBClusters))
	}

	desc := resp.DBClusters[0]
	db.HostedAt = &postgres.Database_Endpoint{
		Address: *desc.Endpoint,
		Port:    uint32(*desc.Port),
	}

	if _, err := ec2cli.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:    aws.String(groupId),
		FromPort:   desc.Port,
		ToPort:     desc.Port,
		IpProtocol: aws.String("tcp"),
		CidrIp:     aws.String(ipRange),
	}); err != nil {
		// Apparently there's no nicer type for this.
		var e smithy.APIError
		if errors.As(err, &e) && e.ErrorCode() == "InvalidPermission.Duplicate" {
			log.Printf("Ingress for security group %s is already authorized for port %d", groupId, *desc.Port)
			// TODO update?
		} else {
			return fmt.Errorf("failed to add permissions to security group: %v", err)
		}
	}
	log.Printf("Authorized security group ingress for port %d", *desc.Port)

	// Wait for endpoints to be ready
wait:
	for {
		// TODO watch would be nice
		time.Sleep(connBackoff)

		resp, err := rdscli.DescribeDBClusterEndpoints(ctx, &awsrds.DescribeDBClusterEndpointsInput{
			DBClusterIdentifier: aws.String(id),
		})
		if err != nil {
			return err
		}

		if len(resp.DBClusterEndpoints) == 0 {
			// keep waiting
			continue
		}

		for k, endpoint := range resp.DBClusterEndpoints {
			log.Printf("Endpoint %d has status %s", k, *endpoint.Status)
			if *endpoint.Status != "available" {
				continue wait
			}
		}

		return initcommon.PrepareDatabase(ctx, db)
	}
}

func createDBSubnetGroup(ctx context.Context, envName string, rdscli *awsrds.Client, ec2cli *ec2.Client, vpcId string) (string, error) {
	resp, err := ec2cli.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []ec2types.Filter{{
			Name:   aws.String("vpc-id"),
			Values: []string{vpcId},
		}},
	})
	if err != nil {
		return "", fmt.Errorf("unable to list subnets for VPC %s: %v", vpcId, err)
	}

	if len(resp.Subnets) == 0 {
		return "", fmt.Errorf("found no subnets for VPC %s", vpcId)
	}

	var subnetIds []string
	for _, subnet := range resp.Subnets {
		subnetIds = append(subnetIds, *subnet.SubnetId)
	}

	name := fmt.Sprintf("ns-%s-db-subnet", envName)
	if _, err := rdscli.CreateDBSubnetGroup(ctx, &awsrds.CreateDBSubnetGroupInput{
		DBSubnetGroupName:        aws.String(name),
		DBSubnetGroupDescription: aws.String(fmt.Sprintf("Namespace DB Subnet group for RDS deployments in %s environment.", envName)),
		SubnetIds:                subnetIds, // TODO Should we create our own?
	}); err != nil {
		var e *rdstypes.DBSubnetGroupAlreadyExistsFault
		if errors.As(err, &e) {
			log.Printf("RDS DB subnet group %s already exists", name)
			return name, nil
		} else {
			return "", fmt.Errorf("failed to create db subnet group: %v", err)
		}
	}

	log.Printf("Created DB subnet group %q", name)
	return name, nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	flag.Parse()

	log.Printf("postgres init begins")
	ctx := context.Background()

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

	dbs, err := initcommon.ReadConfigs()
	if err != nil {
		log.Fatalf("%v", err)
	}

	eg, ctx := errgroup.WithContext(ctx)
	for _, db := range dbs {
		db := db // Close db
		eg.Go(func() error {
			return prepareCluster(ctx, *envName, *vpcID, rdscli, ec2cli, dbGroup, db)
		})
	}

	if err := eg.Wait(); err != nil {
		log.Fatal(err)
	}

	log.Printf("postgres init completed")
}
