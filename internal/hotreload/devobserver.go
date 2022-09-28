// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package hotreload

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/internal/fnfs/workspace/wsremote"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/internal/wscontents"
	"namespacelabs.dev/foundation/provision/parsed"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type FileSyncDevObserver struct {
	log          io.Writer
	server       runtime.Deployable
	cluster      runtime.ClusterNamespace
	fileSyncPort int32

	mu   sync.Mutex
	conn *grpc.ClientConn
}

func NewFileSyncDevObserver(ctx context.Context, cluster runtime.ClusterNamespace, srv parsed.Server, fileSyncPort int32) *FileSyncDevObserver {
	return &FileSyncDevObserver{
		log:          console.TypedOutput(ctx, "hot reload", console.CatOutputUs),
		server:       srv.Proto(),
		cluster:      cluster,
		fileSyncPort: fileSyncPort,
	}
}

func (do *FileSyncDevObserver) Close() error {
	do.mu.Lock()
	defer do.mu.Unlock()
	return do.cleanup()
}

func (do *FileSyncDevObserver) cleanup() error {
	var errs []error

	if do.conn != nil {
		if err := do.conn.Close(); err != nil {
			errs = append(errs, err)
		}
		do.conn = nil
	}

	return multierr.New(errs...)
}

func (do *FileSyncDevObserver) OnDeployment(ctx context.Context) {
	do.mu.Lock()
	defer do.mu.Unlock()

	err := do.cleanup()
	if err != nil {
		fmt.Fprintln(do.log, "failed to port forwarding cleanup", err)
	}

	orch := compute.On(ctx)
	sink := tasks.SinkFrom(ctx)

	// A background context is used here as the connection we create will be
	// long-lived. The parent orchestrator and sink are then patched in when an
	// actual connection attempt is made.
	ctxWithTimeout, done := context.WithTimeout(context.Background(), 15*time.Second)
	defer done()

	t := time.Now()

	conn, err := grpc.DialContext(ctxWithTimeout, "filesync-"+do.server.GetName(),
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			patchedContext := compute.AttachOrch(tasks.WithSink(ctx, sink), orch)

			return do.cluster.DialServer(patchedContext, do.server, do.fileSyncPort)
		}),
	)
	if err != nil {
		fmt.Fprintln(do.log, "failed to connect to filesync", err)
		return
	}

	do.conn = conn

	fmt.Fprintf(do.log, "Connected to FileSync (for hot reload), took %v.\n", time.Since(t))
}

func (do *FileSyncDevObserver) Deposit(ctx context.Context, s *wsremote.Signature, fe []*wscontents.FileEvent) (bool, error) {
	do.mu.Lock()
	defer do.mu.Unlock()

	if do.conn == nil {
		return false, nil
	}

	var paths uniquestrings.List
	for _, r := range fe {
		paths.Add(r.Path)
	}

	fmt.Fprintf(do.log, "FileSync event: %s, paths: %s\n", s, strings.Join(paths.Strings(), ", "))

	newCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if _, err := wsremote.NewFileSyncServiceClient(do.conn).Push(newCtx, &wsremote.PushRequest{
		Signature: s,
		FileEvent: fe,
	}); err != nil {
		return false, err
	}

	return true, nil
}
