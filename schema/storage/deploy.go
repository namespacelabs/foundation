// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package storage

import (
	"namespacelabs.dev/foundation/internal/planning/constants"
)

func (e *Endpoint) IsIngress() bool {
	return e != nil && e.EndpointOwner == "" && e.ServiceName == constants.IngressServiceName
}
