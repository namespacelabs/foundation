// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package grpcstdio

import (
	"context"
	"io"
	"log"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/grpcstdio/testdata"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}

func TestProtocol(t *testing.T) {
	ar, aw := io.Pipe()
	br, bw := io.Pipe()

	ctx := context.Background()

	eg, wait := executor.New(ctx)

	s := grpc.NewServer()

	eg.Go(func(ctx context.Context) error {
		x, err := NewSession(ctx, ar, bw)
		if err != nil {
			return err
		}

		testdata.RegisterTestServiceServer(s, &impl{})

		return s.Serve(x.Listener())
	})

	eg.Go(func(ctx context.Context) error {
		y, err := NewSession(ctx, br, aw)
		if err != nil {
			return err
		}

		conn, err := grpc.DialContext(ctx, "stdio",
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithReadBufferSize(0),
			grpc.WithWriteBufferSize(0),
			grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
				return y.Dial(nil)
			}))
		if err != nil {
			return err
		}

		defer conn.Close()

		for k := 0; k < 3; k++ {
			log.Println("will issue make")
			_, err = testdata.NewTestServiceClient(conn).Make(ctx, &testdata.Request{})
			log.Println("got a make response")
		}

		eg.Go(func(ctx context.Context) error {
			s.GracefulStop()
			return nil
		})

		return err
	})

	if err := wait(); err != nil {
		t.Fatal(err)
	}
}

type impl struct {
}

func (i *impl) Make(ctx context.Context, r *testdata.Request) (*testdata.Response, error) {
	log.Println("hello from make")
	return &testdata.Response{}, nil
}
