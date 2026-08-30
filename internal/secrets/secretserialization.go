// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package secrets

import (
	"path/filepath"

	"google.golang.org/protobuf/encoding/protojson"
	"namespacelabs.dev/foundation/framework/kubernetes/kubenaming"
	runtimepb "namespacelabs.dev/foundation/library/runtime"
	"namespacelabs.dev/foundation/schema"
)

const SecretBaseMountPath = "/namespace/secrets"

type Serialized struct {
	RelPath string
	JSON    []byte
}

func RelPath(res *schema.PackageRef) string {
	return kubenaming.DomainFragLike(res.PackageName, res.Name)
}

func Serialize(res *schema.PackageRef) (*Serialized, error) {
	relPath := RelPath(res)

	instance := &runtimepb.SecretInstance{
		Path: filepath.Join(SecretBaseMountPath, relPath),
	}

	serializedInstance, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(instance)
	if err != nil {
		return nil, err
	}

	return &Serialized{
		RelPath: relPath,
		JSON:    serializedInstance,
	}, nil
}
