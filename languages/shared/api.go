// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package shared

import "namespacelabs.dev/foundation/workspace"

type ServerData struct {
	Services []EmbeddedServiceData
}

type EmbeddedServiceData struct {
	Location workspace.Location
}
