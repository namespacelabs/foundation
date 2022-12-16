package postgres

import (
	"context"
	"log"

	"github.com/jackc/pgx/v4/pgxpool"
	"namespacelabs.dev/foundation/framework/resources"
	postgrespb "namespacelabs.dev/foundation/library/database/postgres"
)

// Connect to the Postgress DB resource
func ConnectToResource(ctx context.Context, res *resources.Parsed, resourceRef string) (*DB, error) {
	db := &postgrespb.DatabaseInstance{}
	if err := res.Unmarshal(resourceRef, db); err != nil {
		log.Fatal(err)
	}

	config, err := pgxpool.ParseConfig(db.ConnectionUri)
	if err != nil {
		return nil, err
	}

	// Only connect when the pool starts to be used.
	config.LazyConnect = true

	conn, err := pgxpool.ConnectConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	return NewDB(conn, nil), nil
}
