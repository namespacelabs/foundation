// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package nsboot

import (
	"encoding/json"
	"os"

	"google.golang.org/protobuf/encoding/protojson"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema/storage"
)

func GetBootVersion() (*storage.NamespaceBinaryVersion, error) {
	versionJSON, have := os.LookupEnv("NSBOOT_VERSION")
	if !have {
		return nil, nil
	}
	var errorStruct struct {
		Err string `json:"error"`
	}
	_ = json.Unmarshal([]byte(versionJSON), &errorStruct)
	if errorStruct.Err != "" {
		return nil, fnerrors.InternalError(errorStruct.Err)
	}
	r := &storage.NamespaceBinaryVersion{}
	if err := protojson.Unmarshal([]byte(versionJSON), r); err != nil {
		return nil, fnerrors.InternalError("malformed NSBOOT_VERSION in environment: %w", err)
	}
	return r, nil
}
