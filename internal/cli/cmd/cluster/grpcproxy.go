// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"io"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api/private"
	"namespacelabs.dev/go-ids"

	instancev1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/private/instance"
	controlapi "github.com/moby/buildkit/api/services/control"
)

func serveGRPCProxy(ctx context.Context, workerInfo *controlapi.ListWorkersResponse, listener net.Listener, proxyStatus *proxyStatusDesc, connect func(context.Context) (net.Conn, error)) error {
	p, err := newGrpcProxy(ctx, workerInfo, proxyStatus, connect)
	if err != nil {
		p.proxyStatus.setLastError(ProxyStatus_Failing, err)
		return err
	}

	p.proxyStatus.setStatus(ProxyStatus_Running)
	return p.server.Serve(listener)
}

type grpcProxy struct {
	connect     func(context.Context) (net.Conn, error)
	server      *grpc.Server
	workerInfo  *controlapi.ListWorkersResponse
	proxyStatus *proxyStatusDesc
	instanceCli *private.InstanceServiceClient

	mu sync.Mutex
	// Fields protected by mutex go below
	backendClient *grpc.ClientConn
}

func newGrpcProxy(ctx context.Context, workerInfo *controlapi.ListWorkersResponse, proxyStatus *proxyStatusDesc, connect func(context.Context) (net.Conn, error)) (*grpcProxy, error) {
	instanceCli, err := private.MakeInstanceClient()
	if err != nil {
		console.DebugWithTimestamp(ctx, "failed to create instance client: %v\n", err)
		// Continue running, we'll skip sending ref attachments to guest instance service
	}

	g := &grpcProxy{
		connect:     connect,
		workerInfo:  workerInfo,
		proxyStatus: proxyStatus,
		instanceCli: instanceCli,
	}

	g.server = grpc.NewServer(grpc.UnknownServiceHandler(g.handler))
	return g, nil
}

func (g *grpcProxy) newBackendClient(ctx context.Context, id string) (*grpc.ClientConn, error) {
	g.proxyStatus.setStatus(ProxyStatus_Running)

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.backendClient != nil {
		connState := g.backendClient.GetState()
		if connState == connectivity.Ready || connState == connectivity.Connecting {
			console.DebugWithTimestamp(ctx, "[%s] reused grpc connection: %v\n", id, connState)
			return g.backendClient, nil
		}

		closingErr := g.backendClient.Close()
		console.DebugWithTimestamp(ctx, "[%s] cached grpc connection invalidated: %v, closing err: %v\n", id, connState, closingErr)
		g.backendClient = nil
	}

	client, err := grpc.DialContext(ctx, "",
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			// gRPC server default minimum is 5m, more frequent keepalives can cause "too_many_pings" error
			Time:    time.Minute * 5,
			Timeout: time.Second * 30,
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return g.connect(ctx)
		}))
	if err != nil {
		return nil, err
	}

	console.DebugWithTimestamp(ctx, "[%s] created new grpc connection\n", id)

	g.backendClient = client
	return client, nil
}

func (g *grpcProxy) handler(srv interface{}, serverStream grpc.ServerStream) error {
	g.proxyStatus.incRequest()

	ctx := serverStream.Context()
	fullMethodName, ok := grpc.MethodFromServerStream(serverStream)
	if !ok {
		err := status.Errorf(codes.Internal, "reading method failed")
		console.DebugWithTimestamp(ctx, "reading method failed: %v\n", err)
		return err
	}

	id := ids.NewRandomBase32ID(4)
	console.DebugWithTimestamp(ctx, "[%s] handler %s\n", id, fullMethodName)

	if fullMethodName == "/moby.buildkit.v1.Control/ListWorkers" && g.workerInfo != nil {
		return shortcutListWorkers(ctx, id, g.workerInfo, serverStream)
	}

	md, _ := metadata.FromIncomingContext(serverStream.Context())
	outgoingCtx := metadata.NewOutgoingContext(serverStream.Context(), md.Copy())
	backendConn, err := g.newBackendClient(outgoingCtx, id)
	if err != nil {
		console.DebugWithTimestamp(ctx, "[%s] creating backend connection failed: %v\n", id, err)
		err := status.Errorf(codes.Internal, "failed to connect to backend: %v", err)
		g.proxyStatus.setLastError(ProxyStatus_Failing, err)
		return err
	}

	clientStreamDescForProxying := &grpc.StreamDesc{
		ServerStreams: true,
		ClientStreams: true,
	}

	clientCtx, clientCancel := context.WithCancel(outgoingCtx)
	defer clientCancel()

	clientStream, err := grpc.NewClientStream(clientCtx, clientStreamDescForProxying, backendConn, fullMethodName)
	if err != nil {
		console.DebugWithTimestamp(ctx, "[%s] failed to create client stream: %v\n", id, err)
		err := status.Errorf(codes.Internal, "failed create client: %v", err)
		g.proxyStatus.setLastError(ProxyStatus_Failing, err)
		return err
	}

	s2cInterceptors := map[string]proxyFunc{
		"/moby.buildkit.v1.Control/Solve": shortcutSolveRequest(g.instanceCli),
	}

	s2cErrChan := proxyServerToClient(ctx, id, fullMethodName, serverStream, clientStream, s2cInterceptors)
	c2sErrChan := proxyClientToServer(ctx, id, fullMethodName, clientStream, serverStream, nil)
	// Make sure to close both client and server connections
	for i := 0; i < 2; i++ {
		select {
		case s2cErr := <-s2cErrChan:
			s2cErrChan = nil // Receive on closed channel does not block, set to nil
			if s2cErr == io.EOF {
				clientStream.CloseSend()
			} else {
				clientCancel()
				console.DebugWithTimestamp(ctx, "[%s] failed proxying s2c: %v\n", id, s2cErr)
				err := status.Errorf(codes.Internal, "failed proxying s2c: %v", s2cErr)
				g.proxyStatus.setLastError(ProxyStatus_Failing, err)
				return err
			}

		case c2sErr := <-c2sErrChan:
			c2sErrChan = nil // Receive on closed channel does not block, set to nil
			serverStream.SetTrailer(clientStream.Trailer())
			// c2sErr will contain RPC error from client code. If not io.EOF return the RPC error as server stream error.
			if c2sErr != io.EOF {
				console.DebugWithTimestamp(ctx, "[%s] failed proxying c2s: %v\n", id, c2sErr)
				g.proxyStatus.setLastError(ProxyStatus_Failing, c2sErr)
				return err
			}

			// Happy case
			return nil

		case <-ctx.Done():
			err := ctx.Err()
			console.DebugWithTimestamp(ctx, "[%s] server stream context is done: %v\n", id, err)
			g.proxyStatus.setLastError(ProxyStatus_Failing, err)
			return err
		}
	}

	return status.Errorf(codes.Internal, "[%s] gRPC proxy should never reach this stage.", id)
}

func proxyClientToServer(ctx context.Context, id, fullMethodName string, src grpc.ClientStream, dst grpc.ServerStream, interceptorsMap map[string]proxyFunc) chan error {
	ret := make(chan error, 1)
	go func() {
		defer close(ret)
		// Server headers are only readable after first client msg is
		// received but must be written to server stream before the first msg is sent
		if err := propagateHeaders(src, dst); err != nil {
			ret <- err
			return
		}

		if err := doProxy(ctx, id, fullMethodName, src, dst, interceptorsMap); err != nil {
			ret <- err
			return
		}
	}()
	return ret
}

func proxyServerToClient(ctx context.Context, id, fullMethodName string, src grpc.ServerStream, dst grpc.ClientStream, interceptorsMap map[string]proxyFunc) chan error {
	ret := make(chan error, 1)
	go func() {
		defer close(ret)
		if err := doProxy(ctx, id, fullMethodName, src, dst, interceptorsMap); err != nil {
			ret <- err
		}
	}()
	return ret
}

func shortcutListWorkers(ctx context.Context, id string, workerInfo *controlapi.ListWorkersResponse, dst grpc.ServerStream) error {
	md := map[string][]string{
		"content-type": {"application/grpc"},
	}

	if err := dst.SendHeader(md); err != nil {
		return err
	}

	if err := dst.SendMsg(workerInfo); err != nil {
		return err
	}

	console.DebugWithTimestamp(ctx, "[%s] ListWorkers injected worker info\n", id)

	return nil
}

func shortcutSolveRequest(instanceCli *private.InstanceServiceClient) proxyFunc {
	return func(ctx context.Context, id, fullMethodName string, src Stream, dst Stream) error {
		solveReq := &controlapi.SolveRequest{}
		if err := src.RecvMsg(solveReq); err != nil {
			return err
		}

		console.DebugWithTimestamp(ctx, "[%s] shortcutSolveRequest: %v\n", id, solveReq.String())
		if instanceCli != nil {
			if _, err := instanceCli.AddAttachment(ctx, &instancev1beta.AddAttachmentRequest{
				BuildAttachment: &instancev1beta.BuildAttachment{
					BuildRef: solveReq.GetRef(),
				},
			},
			); err != nil {
				console.DebugWithTimestamp(ctx, "[%s] AddAttachment failed with: %v\n", id, err)
				return err
			}
		}

		if err := dst.SendMsg(solveReq); err != nil {
			return err
		}

		return nil
	}
}

func propagateHeaders(src grpc.ClientStream, dst grpc.ServerStream) error {
	f := &emptypb.Empty{}
	if err := src.RecvMsg(f); err != nil {
		return err // this can be io.EOF which is happy case
	}

	// Server headers are only readable after first client msg is
	// received but must be written to server stream before the first msg is sent
	md, err := src.Header()
	if err != nil {
		return err
	}

	if err := dst.SendHeader(md); err != nil {
		return err
	}

	if err := dst.SendMsg(f); err != nil {
		return err
	}

	return nil
}

type Stream interface {
	SendMsg(m interface{}) error
	RecvMsg(m interface{}) error
}

type proxyFunc func(context.Context, string, string, Stream, Stream) error

func doProxy(ctx context.Context, id, fullMethodName string, src Stream, dst Stream, interceptorsMap map[string]proxyFunc) error {
	for {
		if cb, ok := interceptorsMap[fullMethodName]; ok {
			if err := cb(ctx, id, fullMethodName, src, dst); err != nil {
				return err
			}
		}

		// Not really empty message. All fields are unmarshalled in Empty.unknownFields.
		f := &emptypb.Empty{}
		if err := src.RecvMsg(f); err != nil {
			return err
		}

		if err := dst.SendMsg(f); err != nil {
			return err
		}
	}
}
