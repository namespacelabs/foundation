// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package multidb

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v4"
	"google.golang.org/protobuf/types/known/emptypb"
	"namespacelabs.dev/foundation/framework/resources"
	"namespacelabs.dev/foundation/internal/testdata/service/proto"
	"namespacelabs.dev/foundation/library/database/postgres"
	"namespacelabs.dev/foundation/std/go/server"
	oldpostgres "namespacelabs.dev/foundation/universe/db/postgres"
)

type Service struct {
	postgres *pgx.Conn
	rds      *oldpostgres.DB
}

const timeout = 2 * time.Second

func addOldPostgres(ctx context.Context, db *oldpostgres.DB, item string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	_, err := db.Exec(ctx, "INSERT INTO list (Item) VALUES ($1);", item)
	return err
}

func (svc *Service) AddRds(ctx context.Context, req *proto.AddRequest) (*emptypb.Empty, error) {
	log.Printf("new AddRds request: %+v\n", req)

	if err := addOldPostgres(ctx, svc.rds, req.Item); err != nil {
		log.Fatalf("failed to add list item: %v", err)
	}

	response := &emptypb.Empty{}
	return response, nil
}

func addPostgres(ctx context.Context, conn *pgx.Conn, item string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	_, err := conn.Exec(ctx, "INSERT INTO list (Item) VALUES ($1);", item)
	return err
}

func (svc *Service) AddPostgres(ctx context.Context, req *proto.AddRequest) (*emptypb.Empty, error) {
	log.Printf("new AddPostgres request: %+v\n", req)

	if err := addPostgres(ctx, svc.postgres, req.Item); err != nil {
		log.Fatalf("failed to add list item: %v", err)
	}

	response := &emptypb.Empty{}
	return response, nil
}

func listOldPostgres(ctx context.Context, db *oldpostgres.DB) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rows, err := db.Query(ctx, "SELECT Item FROM list;")
	if err != nil {
		return nil, fmt.Errorf("failed read list: %w", err)
	}
	defer rows.Close()

	var res []string
	for rows.Next() {
		var item string
		err = rows.Scan(&item)
		if err != nil {
			return nil, err
		}
		res = append(res, item)
	}

	return res, nil
}

func listPostgres(ctx context.Context, conn *pgx.Conn) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rows, err := conn.Query(ctx, "SELECT Item FROM list;")
	if err != nil {
		return nil, fmt.Errorf("failed read list: %w", err)
	}
	defer rows.Close()

	var res []string
	for rows.Next() {
		var item string
		err = rows.Scan(&item)
		if err != nil {
			return nil, err
		}
		res = append(res, item)
	}

	return res, nil
}

func (svc *Service) List(ctx context.Context, _ *emptypb.Empty) (*proto.ListResponse, error) {
	log.Print("new List request\n")

	var list []string

	rdslist, err := listOldPostgres(ctx, svc.rds)
	if err != nil {
		log.Fatalf("failed to read list: %v", err)
	}
	list = append(list, rdslist...)

	pglist, err := listPostgres(ctx, svc.postgres)
	if err != nil {
		log.Fatalf("failed to read list: %v", err)
	}
	list = append(list, pglist...)

	response := &proto.ListResponse{Item: list}
	return response, nil
}

func WireService(ctx context.Context, srv server.Registrar, deps ServiceDeps) {
	svc := &Service{
		rds: deps.Rds,
	}
	var err error
	svc.postgres, err = wirePostgres()
	if err != nil {
		log.Fatalf("failed to wire postgres: %v", err)
	}
	proto.RegisterMultiDbListServiceServer(srv, svc)
}

const postgresDbRef = "namespacelabs.dev/foundation/internal/testdata/service/multidb:postgres"

func wirePostgres() (*pgx.Conn, error) {
	ctx := context.Background()

	resources, err := resources.LoadResources()
	if err != nil {
		log.Fatal(err)
	}

	db := &postgres.DatabaseInstance{}
	if err := resources.Unmarshal(postgresDbRef, db); err != nil {
		log.Fatal(err)
	}

	conn, err := pgx.Connect(ctx, fmt.Sprintf("postgres://postgres:%s@%s/%s", db.Cluster.Password, db.Cluster.Url, db.Name))
	if err != nil {
		log.Fatal(err)
	}

	return conn, nil
}
