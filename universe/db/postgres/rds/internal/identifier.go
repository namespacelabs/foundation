// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package internal

import (
	"fmt"
)

func ClusterIdentifier(env, dbName string) string {
	return fmt.Sprintf("ns-%s-postgres-%s", env, dbName)
}
