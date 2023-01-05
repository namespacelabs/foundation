// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package testing

import (
	"fmt"
	"strings"

	"namespacelabs.dev/foundation/schema"
)

func MakeHttpUrl(endpoint *schema.Endpoint, subPath string) string {
	return fmt.Sprintf("http://%s:%d/%s", endpoint.AllocatedName, endpoint.ExportedPort, strings.TrimPrefix(subPath, "/"))
}
