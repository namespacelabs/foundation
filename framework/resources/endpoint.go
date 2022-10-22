// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package resources

import "namespacelabs.dev/foundation/framework/runtime"

func LookupServerEndpoint(resources *Parsed, serverRef, service string) (string, error) {
	srv := &runtime.Server{}
	if err := resources.Unmarshal(serverRef, &srv); err != nil {
		return "", err
	}

	return runtime.Endpoint(srv, service)
}
