// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package internal

import "fmt"

func ClusterIdentifier(dbName string) string {
	return fmt.Sprintf("ns-postgres-%s", dbName)
}
