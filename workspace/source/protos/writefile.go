// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package protos

import (
	"os"
	"path/filepath"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func WriteFile(path string, msg proto.Message) error {
	serialized, err := proto.MarshalOptions{Deterministic: true}.Marshal(msg)
	if err != nil {
		return fnerrors.New("failed to marshal: %w", err)
	}

	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fnerrors.New("mkdir: failed: %w", err)
	}

	if err := os.WriteFile(path, serialized, 0644); err != nil {
		return fnerrors.New("failed to write %q: %w", path, err)
	}

	return nil
}
