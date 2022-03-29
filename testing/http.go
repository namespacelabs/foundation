// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package testing

import (
	"fmt"
	"strings"

	"namespacelabs.dev/foundation/schema"
)

func MakeHttpUrl(endpoint *schema.Endpoint, subPath string) string {
	return fmt.Sprintf("http://%s:%d/%s", endpoint.AllocatedName, endpoint.Port.ContainerPort, strings.TrimPrefix(subPath, "/"))
}