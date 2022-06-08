// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/bradleyjkemp/cupaloy"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"gotest.tools/assert"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/languages/nodejs/grpcgen"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

const (
	testFile1 = "test1.proto"
)

var (
	testFiles = []string{
		testFile1,
		"nested/test2.proto",
	}
)

func TestCodegen(t *testing.T) {
	var fsys memfs.FS
	for _, fn := range testFiles {
		file, err := os.ReadFile(fn)
		assert.NilError(t, err)
		fsys.Add(fn, file)
	}

	fds, err := protos.Parse(&fsys, testFiles)
	assert.NilError(t, err)

	for _, fd := range fds.File {
		if fd.GetName() != testFile1 {
			continue
		}

		assertCodegen(t, fd, fds, "servers-clients", grpcgen.GenOpts{GenClients: true, GenServers: true})
		assertCodegen(t, fd, fds, "servers", grpcgen.GenOpts{GenServers: true})
		assertCodegen(t, fd, fds, "clients", grpcgen.GenOpts{GenClients: true})
	}
}

func assertCodegen(t *testing.T, fd *descriptor.FileDescriptorProto, fds *protos.FileDescriptorSetAndDeps, suffix string, opts grpcgen.GenOpts) {
	generatedCode, err := grpcgen.Generate(fd, fds, opts)
	assert.NilError(t, err)
	// Adding ".generated" extension so the ".ts" files don't appear broken (due to missing dependencies) in the IDE.
	assert.NilError(t, cupaloy.SnapshotMulti(fmt.Sprintf("%s-%s.ts.generated", filepath.Base(fd.GetName()), suffix), generatedCode))
}
