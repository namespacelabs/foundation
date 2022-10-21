// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package resources

import "namespacelabs.dev/foundation/framework/runtime"

func LookupServerEndpoint(resources *Parsed, serverRef, service string) (string, error) {
	srv := &runtime.Server{}
	if err := resources.Unmarshal(serverRef, &srv); err != nil {
		return "", err
	}

	return runtime.Endpoint(srv, service)
}
