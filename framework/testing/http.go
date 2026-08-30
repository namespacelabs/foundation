// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package testing

import (
	"fmt"
	"log"
	"strings"

	"namespacelabs.dev/foundation/schema"
)

func MakeHttpUrl(endpoint *schema.Endpoint, subPath string) string {
	if len(endpoint.Ports) == 0 {
		log.Fatal("endpoint has no ports")
	}

	return fmt.Sprintf("http://%s:%d/%s", endpoint.AllocatedName, endpoint.Ports[0].ExportedPort, strings.TrimPrefix(subPath, "/"))
}
