// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package postgres

import (
	"fmt"
)

type ClusterInstance interface {
	GetUser() string
	GetPassword() string
	GetAddress() string
	GetSslMode() string
}

func ConnectionUri(cluster ClusterInstance, db string) string {
	uri := fmt.Sprintf("postgres://%s:%s@%s/%s", UserOrDefault(cluster.GetUser()), cluster.GetPassword(), cluster.GetAddress(), db)

	if cluster.GetSslMode() != "" {
		uri = fmt.Sprintf("%s?sslmode=%s", uri, cluster.GetSslMode())
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
