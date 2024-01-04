// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package rawlistener

import (
	"context"
	"net"
	"net/http"

	"namespacelabs.dev/foundation/std/go/grpc/servercore"
	"namespacelabs.dev/foundation/std/go/server"
)

func init() {
	servercore.SetListenerConfiguration("second", servercore.DefaultConfiguration{})
}

func WireService(ctx context.Context, srv server.Registrar, deps ServiceDeps) {
	srv.RegisterListener(func(ctx context.Context, lst net.Listener) error {
		s := servercore.NewHttp2CapableServer(http.NewServeMux(), servercore.HTTPOptions{})
		return servercore.ListenAndGracefullyShutdownHTTP(ctx, "second", s, lst)
	})
}
