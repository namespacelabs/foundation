package postgres

import (
	"fmt"

	postgresclass "namespacelabs.dev/foundation/library/database/postgres"
)

func ConnectionUri(cluster *postgresclass.ClusterInstance, db string) string {
	uri := fmt.Sprintf("postgres://%s:%s@%s/%s", UserOrDefault(cluster.User), cluster.Password, cluster.Address, db)

	if cluster.SslMode != "" {
		uri = fmt.Sprintf("%s?sslmode=%s", uri, cluster.SslMode)
	}

	return uri
}

// Ensure backwards compatibility
func UserOrDefault(user string) string {
	if user != "" {
		return user

	}

	return "postgres"
}
