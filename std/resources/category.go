// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package resources

import (
	"fmt"

	schema "namespacelabs.dev/foundation/schema"
)

func ResourceInstanceCategory(id string) string {
	return fmt.Sprintf("resourceinstance:%s", id)
}

func ResourceID(resourceRef *schema.PackageRef) string {
	return fmt.Sprintf("%s:%s", resourceRef.AsPackageName(), resourceRef.Name)
}
