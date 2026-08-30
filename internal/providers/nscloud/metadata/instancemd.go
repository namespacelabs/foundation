// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package metadata

import (
	"encoding/json"
	"os"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

type InstanceMetadata struct {
	Version          string `json:"version,omitempty"`
	InstanceEndpoint string `json:"instance_endpoint,omitempty"`
	Certs            struct {
		PublicPemPath     string `json:"public_pem_path,omitempty"`
		PrivateKeyPath    string `json:"private_key_path,omitempty"`
		HostPublicPemPath string `json:"host_public_pem_path,omitempty"`
	} `json:"certs,omitempty"`
}

func InstanceMetadataFromFile() (InstanceMetadata, error) {
	metadataPath := os.Getenv("NSC_METADATA_FILE")
	if metadataPath == "" {
		var err error
		metadataPath, err = MetadataFile()
		if err != nil {
			return InstanceMetadata{}, err
		}
	}

	var md InstanceMetadata
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return md, err
	}

	//XXX check version first, then unmarshal to right struct

	if err := json.Unmarshal(data, &md); err != nil {
		return md, fnerrors.Newf("instance metadata is invalid: %w", err)
	}

	return md, nil
}
