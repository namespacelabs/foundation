// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package integration

import (
	"context"
	"io"
	"strings"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/languages/nodejs/grpcgen"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

func generateGrpcApi(ctx context.Context, protos *protos.FileDescriptorSetAndDeps, loc pkggraph.Location) error {
	for _, file := range protos.GetFile() {
		generatedCode, err := grpcgen.Generate(file, protos, grpcgen.GenOpts{
			GenClients: true,
			GenServers: true,
		})
		if err != nil {
			return err
		}

		// Not generating the file if there are no services.
		if generatedCode != nil {
			filename := generatedGrpcFilePath(file.GetName())
			if err := fnfs.WriteWorkspaceFile(ctx, console.Stdout(ctx), loc.Module.ReadWriteFS(), filename, func(w io.Writer) error {
				_, err := w.Write(generatedCode)
				return err
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

// Relative to the module root.
func generatedGrpcFilePath(protoFn string) string {
	return strings.TrimSuffix(protoFn, ".proto") + "_grpc.fn.ts"
}
