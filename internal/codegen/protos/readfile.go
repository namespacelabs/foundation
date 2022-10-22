// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package protos

import (
	"os"
	"reflect"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func ReadFileAndBytes[V proto.Message](path string) (V, []byte, error) {
	var empty V

	bytes, err := os.ReadFile(path)
	if err != nil {
		return empty, nil, fnerrors.New("%s: failed to load: %w", path, err)
	}

	s := reflect.New(reflect.TypeOf(empty).Elem()).Interface().(V)

	if err := proto.Unmarshal(bytes, s); err != nil {
		return empty, nil, fnerrors.New("%s: unmarshal failed: %w", path, err)
	}

	return s, bytes, nil
}

func ReadFile[V proto.Message](path string) (V, error) {
	v, _, err := ReadFileAndBytes[V](path)
	return v, err
}
