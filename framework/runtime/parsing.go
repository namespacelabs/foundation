// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import (
	"fmt"

	"namespacelabs.dev/foundation/schema/runtime"
)

type Server = runtime.Server

func Endpoint(srv *Server, name string) (string, error) {
	for _, s := range srv.Service {
		if s.Name == name {
			return s.Endpoint, nil
		}
	}

	return "", fmt.Errorf("endpoint %s not found for server %s", name, srv.PackageName)
}

func ServerEndpoint(rtcfg *runtime.RuntimeConfig, pkg, name string) (string, error) {
	for _, e := range rtcfg.StackEntry {
		if e.PackageName == pkg {
			return Endpoint(e, name)
		}
	}

	return "", fmt.Errorf("server %s not found in runtime config stack", pkg)
}
