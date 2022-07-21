// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package configure

import (
	"context"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/compression"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/grpcstdio"
)

const (
	maximumWallclockTime = 2 * time.Minute
)

var (
	debug = flag.Bool("debug", false, "If set to true, emits debugging information.")

	inlineInvocation           = flag.String("inline_invocation", "", "If set, calls the method inline on stdin/stdout.")
	inlineInvocationCompressed = flag.Bool("inline_invocation_compressed", false, "If set, the input and output are compressed.")
	inlineInvocationJson       = flag.Bool("inline_invocation_json", false, "If set, the input and output are json.")
	inlineInvocationInput      = flag.String("inline_invocation_input", "", "If set, reads the request from the path instead of os.Stdin.")
	inlineInvocationOutput     = flag.String("inline_invocation_output", "", "If set, writes the result to the path instead of os.Stdout.")
)

func RunServer(ctx context.Context, register func(grpc.ServiceRegistrar)) error {
	go func() {
		if *debug {
			log.Printf("Setup kill-switch: %v", maximumWallclockTime)
		}

		time.Sleep(maximumWallclockTime)
		log.Fatalf("aborting tool after %v", maximumWallclockTime)
	}()

	flag.Parse()

	if *inlineInvocation != "" {
		var reg inlineRegistrar
		register(&reg)
		m := strings.Split(*inlineInvocation, "/")
		if len(m) != 2 {
			log.Fatal("bad invocation specification")
		}

		for _, sr := range reg.registrations {
			if sr.ServiceDesc.ServiceName != m[0] {
				continue
			}

			for _, x := range sr.ServiceDesc.Methods {
				if x.MethodName == m[1] {
					result, err := x.Handler(sr.Impl, context.Background(), decodeInput, nil)
					if err != nil {
						log.Fatal(err)
					}

					if err := marshalOutput(result); err != nil {
						log.Fatal(err)
					}

					os.Exit(0)
				}
			}
		}

		log.Fatalf("%s: don't know how to handle this method", *inlineInvocation)
	}

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

func decodeInput(target interface{}) error {
	var err error
	var bytes []byte

	if *inlineInvocationInput != "" {
		bytes, err = ioutil.ReadFile(*inlineInvocationInput)
	} else {
		bytes, err = io.ReadAll(os.Stdin)
	}

	if err != nil {
		return fnerrors.InternalError("failed to read input: %w", err)
	}

	if *inlineInvocationCompressed {
		payload, err := compression.DecompressZstd(bytes)
		if err != nil {
			return fnerrors.InternalError("failed to decompress payload: %w", err)
		}

		return proto.Unmarshal(payload, target.(proto.Message))
	}

	if *inlineInvocationJson {
		return protojson.Unmarshal(bytes, target.(proto.Message))
	}

	return proto.Unmarshal(bytes, target.(proto.Message))
}

func marshalOutput(out interface{}) error {
	w := os.Stdout
	if *inlineInvocationOutput != "" {
		f, err := os.Create(*inlineInvocationOutput)
		if err != nil {
			return fnerrors.InternalError("failed to create output: %w", err)
		}
		w = f
	}

	var bytes []byte
	var err error

	if *inlineInvocationJson {
		bytes, err = protojson.Marshal(out.(proto.Message))
	} else {
		bytes, err = proto.Marshal(out.(proto.Message))
	}

	if err != nil {
		return fnerrors.InternalError("failed to serialize output: %w", err)
	}

	if *inlineInvocationCompressed {
		w, err := zstd.NewWriter(w)
		if err != nil {
			return fnerrors.InternalError("failed to prepare output: %w", err)
		}
		if _, err := w.Write(bytes); err != nil {
			return fnerrors.InternalError("failed to compress output: %w", err)
		}
		if err := w.Close(); err != nil {
			return fnerrors.InternalError("failed to finalize compression: %w", err)
		}
		return nil
	}

	if _, err := w.Write(bytes); err != nil {
		return fnerrors.InternalError("failed to write output: %w", err)
	}

	if *inlineInvocationOutput != "" {
		if err := w.Close(); err != nil {
			return fnerrors.InternalError("failed to close output: %w", err)
		}
	}

	return nil
}

type inlineRegistrar struct {
	registrations []inlineRegistration
}

type inlineRegistration struct {
	ServiceDesc *grpc.ServiceDesc
	Impl        interface{}
}

func (reg *inlineRegistrar) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
	reg.registrations = append(reg.registrations, inlineRegistration{desc, impl})
}
