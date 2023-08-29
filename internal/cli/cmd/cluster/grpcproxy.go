// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
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
)

var (
	clientStreamDescForProxying = &grpc.StreamDesc{
		ServerStreams: true,
		ClientStreams: true,
	}
)

type grpcConnectCb func(context.Context) (*grpc.ClientConn, error)

func serveGRPCProxy(listener net.Listener, connect func(context.Context) (net.Conn, error)) error {
	p, err := newGrpcProxy(connect)
	if err != nil {
		return err
	}

	return p.Serve(listener)
}

type grpcProxy struct {
	mu sync.Mutex
	*grpc.Server
	backendClient *grpc.ClientConn
	connect       func(context.Context) (net.Conn, error)
}

func newGrpcProxy(connect func(context.Context) (net.Conn, error)) (*grpcProxy, error) {
	g := &grpcProxy{
		connect: connect,
	}

	s := grpc.NewServer(grpc.UnknownServiceHandler(g.handler))
	g.Server = s
	return g, nil
}

func (g *grpcProxy) newBackendClient(ctx context.Context) (*grpc.ClientConn, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.backendClient != nil {
		if g.backendClient.GetState() == connectivity.Ready {
			fmt.Fprintf(console.Debug(ctx), "reused grpc connection\n")
			return g.backendClient, nil
		}
		fmt.Fprintf(console.Debug(ctx), "cached grpc connection invalidated\n")
		g.backendClient = nil
	}

	client, err := grpc.DialContext(ctx, "",
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    time.Second * 30,
			Timeout: time.Minute,
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return g.connect(ctx)
		}))
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(console.Debug(ctx), "created new grpc connection\n")

	g.backendClient = client
	return client, nil
}

func (g *grpcProxy) handler(srv interface{}, serverStream grpc.ServerStream) error {
	ctx := serverStream.Context()
	fullMethodName, ok := grpc.MethodFromServerStream(serverStream)
	if !ok {
		err := status.Errorf(codes.Internal, "reading method failed")
		fmt.Fprintf(console.Errors(ctx), "reading method failed: %v\n", err)
		return err
	}
	fmt.Fprintf(console.Debug(ctx), "handler %s\n", fullMethodName)

	md, _ := metadata.FromIncomingContext(serverStream.Context())
	outgoingCtx := metadata.NewOutgoingContext(serverStream.Context(), md.Copy())
	backendConn, err := g.newBackendClient(outgoingCtx)
	if err != nil {
		fmt.Fprintf(console.Errors(ctx), "creating backend connection failed: %v\n", err)
		return status.Errorf(codes.Internal, "failed connect to backend: %v", err)
	}

	clientCtx, clientCancel := context.WithCancel(outgoingCtx)
	defer clientCancel()

	clientStream, err := grpc.NewClientStream(clientCtx, clientStreamDescForProxying, backendConn, fullMethodName)
	if err != nil {
		fmt.Fprintf(console.Errors(ctx), "failed to create client stream: %v\n", err)
		return status.Errorf(codes.Internal, "failed create client: %v", err)
	}

	s2cErrChan := g.proxyServerToClient(serverStream, clientStream)
	c2sErrChan := g.proxyClientToServer(clientStream, serverStream)
	// Make sure to close both client and server connections
	for i := 0; i < 2; i++ {
		select {
		case s2cErr := <-s2cErrChan:
			if s2cErr == io.EOF {
				clientStream.CloseSend()
			} else {
				clientCancel()
				fmt.Fprintf(console.Errors(ctx), "failed proxying s2c: %v\n", s2cErr)
				return status.Errorf(codes.Internal, "failed proxying s2c: %v", s2cErr)
			}

		case c2sErr := <-c2sErrChan:

			serverStream.SetTrailer(clientStream.Trailer())
			// c2sErr will contain RPC error from client code. If not io.EOF return the RPC error as server stream error.
			if c2sErr != io.EOF {
				fmt.Fprintf(console.Errors(ctx), "failed proxying c2s: %v\n", c2sErr)
				return c2sErr
			}
			return nil

		case <-ctx.Done():
			err := serverStream.Context().Err()
			fmt.Fprintf(console.Debug(ctx), "server stream context is done: %v\n", err)
			return err
		}
	}

	return status.Errorf(codes.Internal, "gRPC proxy should never reach this stage.")
}

func (g *grpcProxy) proxyClientToServer(src grpc.ClientStream, dst grpc.ServerStream) chan error {
	ret := make(chan error, 1)
	go func() {
		// Not really empty message. All fields are unmarshalled in Empty.unknownFields.
		f := &emptypb.Empty{}
		first := true
		for {
			if err := src.RecvMsg(f); err != nil {
				ret <- err // this can be io.EOF which is happy case
				break
			}

			if first {
				first = false
				// Server headers are only readable after first client msg is
				// received but must be written to server stream before the first msg is sent
				md, err := src.Header()
				if err != nil {
					ret <- err
					break
				}
				if err := dst.SendHeader(md); err != nil {
					ret <- err
					break
				}
			}

			if err := dst.SendMsg(f); err != nil {
				ret <- err
				break
			}
		}
	}()
	return ret
}

func (g *grpcProxy) proxyServerToClient(src grpc.ServerStream, dst grpc.ClientStream) chan error {
	ret := make(chan error, 1)
	go func() {
		// Not really empty message. All fields are unmarshalled in Empty.unknownFields.
		f := &emptypb.Empty{}
		for {
			if err := src.RecvMsg(f); err != nil {
				ret <- err
				break
			}

			if err := dst.SendMsg(f); err != nil {
				ret <- err
				break
			}
		}
	}()
	return ret
}
