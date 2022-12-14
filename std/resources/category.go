// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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

func ScopedID(parentID string, ref *schema.PackageRef) string {
	return JoinID(parentID, ResourceID(ref))
}

func JoinID(parent, child string) string {
	if parent == "" {
		return child
	}
	// Child is kept on the leftside to optimize for readability.
	return fmt.Sprintf("%s;%s", child, parent)
}
