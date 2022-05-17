// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package incluster

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/go/core"

	_ "github.com/go-sql-driver/mysql"
)

var (
	mariadbEndpoint = flag.String("mariadb_endpoint", "", "Endpoint configuration.")
)

func getEndpoint() (*schema.Endpoint, error) {
	if *mariadbEndpoint == "" {
		return nil, errors.New("startup configuration missing, --mariadb_endpoint not specified")
	}

	var endpoint schema.Endpoint
	if err := json.Unmarshal([]byte(*mariadbEndpoint), &endpoint); err != nil {
		return nil, fmt.Errorf("failed to parse mariadb endpoint configuration: %w", err)
	}

	return &endpoint, nil
}

func ProvideDatabase(ctx context.Context, db *Database, deps ExtensionDeps) (*sql.DB, error) {
	endpoint, err := getEndpoint()
	if err != nil {
		return nil, err
	}

	res, err := sql.Open("mysql", fmt.Sprintf("root:%s@tcp(%s:%d)/%s", deps.Creds.Password, endpoint.AllocatedName, endpoint.Port.ContainerPort, db.Name))
	if err != nil {
		panic(err)
	}

	// Asynchronously wait until a database connection is ready.
	deps.ReadinessCheck.RegisterFunc(fmt.Sprintf("%s/%s", core.InstantiationPathFromContext(ctx), db.Name), func(ctx context.Context) error {
		return res.PingContext(ctx)
	})

	return res, nil
}
