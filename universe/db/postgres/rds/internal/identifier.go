// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package internal

import (
	"fmt"
)

func ClusterIdentifier(env, dbName string) string {
	return fmt.Sprintf("ns-%s-postgres-%s", env, dbName)
}
