// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package multidb

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"google.golang.org/protobuf/types/known/emptypb"
	"namespacelabs.dev/foundation/std/go/grpc/server"
)

type Service struct {
	db *pgxpool.Pool
}

const postgresTimeout = 2 * time.Second

func addPostgres(ctx context.Context, db *pgxpool.Pool, item string) error {
	ctx, cancel := context.WithTimeout(ctx, postgresTimeout)
	defer cancel()

	_, err := db.Exec(ctx, "INSERT INTO list (Item) VALUES ($1);", item)
	return err
}

func (svc *Service) AddPostgres(ctx context.Context, req *AddRequest) (*emptypb.Empty, error) {
	log.Printf("new AddPostgres request: %+v\n", req)

	if err := addPostgres(ctx, svc.db, req.Item); err != nil {
		log.Fatalf("failed to add list item: %v", err)
	}

	response := &emptypb.Empty{}
	return response, nil
}

func (svc *Service) AddMaria(ctx context.Context, req *AddRequest) (*emptypb.Empty, error) {
	log.Printf("new AddMaria request: %+v\n", req)

	// TODO

	response := &emptypb.Empty{}
	return response, nil
}

func list(ctx context.Context, db *pgxpool.Pool) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, postgresTimeout)
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

func (svc *Service) List(ctx context.Context, _ *emptypb.Empty) (*ListResponse, error) {
	log.Print("new List request\n")

	l, err := list(ctx, svc.db)
	if err != nil {
		log.Fatalf("failed to read list: %v", err)
	}

	response := &ListResponse{Item: l}
	return response, nil
}

func WireService(ctx context.Context, srv *server.Grpc, deps ServiceDeps) {
	svc := &Service{db: deps.Db}
	RegisterListServiceServer(srv, svc)
}
