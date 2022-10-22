// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package rtypes

import (
	"google.golang.org/protobuf/proto"
	schema "namespacelabs.dev/foundation/schema"
)

type LocalMapping struct {
	// Relative to workspace root.
	LocalPath string `json:"local_path,omitempty"`
	// Absolute path within the host (overrides local_path).
	HostPath string `json:"host_path,omitempty"`
	// Must be an absolute path.
	ContainerPath string `json:"container_path,omitempty"`
}

type ProvisionProps struct {
	ProvisionInput  []ProvisionInput
	Invocation      []*schema.SerializedInvocation
	Extension       []*schema.DefExtension
	ServerExtension []*schema.ServerExtension
}

type ProvisionInput struct {
	Aliases []string // Proto full name.
	Message proto.Message
}
