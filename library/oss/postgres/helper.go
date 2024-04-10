// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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
