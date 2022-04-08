// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package multidb

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"google.golang.org/protobuf/types/known/emptypb"
	"namespacelabs.dev/foundation/std/go/grpc/server"
)

type Service struct {
	maria    *sql.DB
	postgres *pgxpool.Pool
}

const timeout = 2 * time.Second

func addPostgres(ctx context.Context, db *pgxpool.Pool, item string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	_, err := db.Exec(ctx, "INSERT INTO list (Item) VALUES ($1);", item)
	return err
}

func (svc *Service) AddPostgres(ctx context.Context, req *AddRequest) (*emptypb.Empty, error) {
	log.Printf("new AddPostgres request: %+v\n", req)

	if err := addPostgres(ctx, svc.postgres, req.Item); err != nil {
		log.Fatalf("failed to add list item: %v", err)
	}

	response := &emptypb.Empty{}
	return response, nil
}

func addMaria(ctx context.Context, db *sql.DB, item string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	_, err := db.ExecContext(ctx, "INSERT INTO list (Item) VALUES (?);", item)
	return err
}

func (svc *Service) AddMaria(ctx context.Context, req *AddRequest) (*emptypb.Empty, error) {
	log.Printf("new AddMaria request: %+v\n", req)

	if err := addMaria(ctx, svc.maria, req.Item); err != nil {
		log.Fatalf("failed to add list item: %v", err)
	}

	response := &emptypb.Empty{}
	return response, nil
}

func listPostgres(ctx context.Context, db *pgxpool.Pool) ([]string, error) {
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

func listMaria(ctx context.Context, db *sql.DB) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rows, err := db.QueryContext(ctx, "SELECT Item FROM list;")
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

func (svc *Service) List(ctx context.Context, _ *emptypb.Empty) (*ListResponse, error) {
	log.Print("new List request\n")

	pglist, err := listPostgres(ctx, svc.postgres)
	if err != nil {
		log.Fatalf("failed to read list: %v", err)
	}

	marialist, err := listMaria(ctx, svc.maria)
	if err != nil {
		log.Fatalf("failed to read list: %v", err)
	}

	response := &ListResponse{Item: append(pglist, marialist...)}
	return response, nil
}

func WireService(ctx context.Context, srv *server.Grpc, deps *ServiceDeps) {
	svc := &Service{
		maria:    deps.Maria,
		postgres: deps.Postgres,
	}
	RegisterListServiceServer(srv, svc)
}
