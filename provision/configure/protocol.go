// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package configure

import (
	"context"
	"flag"
	"os"

	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/internal/grpcstdio"
)

func RunServer(ctx context.Context, register func(grpc.ServiceRegistrar)) error {
	flag.Parse()

	s := grpc.NewServer()

	x, err := grpcstdio.NewSession(ctx, os.Stdin, os.Stdout, grpcstdio.WithCloseNotifier(func(_ *grpcstdio.Stream) {
		// After we're done replying, shutdown the server, and then the binary.
		// But we can't stop the server from this callback, as we're called with
		// grpcstdio locks held, and terminating the server will need to call
		// Close on open connections, which would lead to a deadlock.
		go s.Stop()
	}))
	if err != nil {
		return err
	}

	register(s)

	return s.Serve(x.Listener())
}
