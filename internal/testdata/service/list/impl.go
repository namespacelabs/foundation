// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package list

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"
	"namespacelabs.dev/foundation/std/go/server"
	"namespacelabs.dev/foundation/internal/testdata/service/proto"
	"namespacelabs.dev/foundation/universe/db/postgres"
)

type Service struct {
	db *postgres.DB
}

const postgresTimeout = 2 * time.Second

func add(ctx context.Context, db *postgres.DB, item string) error {
	ctx, cancel := context.WithTimeout(ctx, postgresTimeout)
	defer cancel()

	_, err := db.Exec(ctx, "INSERT INTO list (Item) VALUES ($1);", item)
	return err
}

func (svc *Service) Add(ctx context.Context, req *proto.AddRequest) (*emptypb.Empty, error) {
	log.Printf("new Add request: %+v\n", req)

	if err := add(ctx, svc.db, req.Item); err != nil {
		log.Fatalf("failed to add list item: %v", err)
	}

	response := &emptypb.Empty{}
	return response, nil
}

func list(ctx context.Context, db *postgres.DB) ([]string, error) {
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

func (svc *Service) List(ctx context.Context, _ *emptypb.Empty) (*proto.ListResponse, error) {
	log.Print("new List request\n")

	l, err := list(ctx, svc.db)
	if err != nil {
		log.Fatalf("failed to read list: %v", err)
	}

	response := &proto.ListResponse{Item: l}
	return response, nil
}

func WireService(ctx context.Context, srv server.Registrar, deps ServiceDeps) {
	svc := &Service{db: deps.Db}
	proto.RegisterListServiceServer(srv, svc)
}
