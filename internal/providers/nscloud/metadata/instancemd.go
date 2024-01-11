package metadata

import (
	"encoding/json"
	"os"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

const (
	wellKnownMetadataPath = "/var/run/nsc/metadata.json"
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
	var md InstanceMetadata
	data, err := os.ReadFile(wellKnownMetadataPath)
	if err != nil {
		return md, err
	}

	//XXX check version first, then unmarshal to right struct

	if err := json.Unmarshal(data, &md); err != nil {
		return md, fnerrors.New("instance metadata is invalid: %w", err)
	}

	return md, nil
}
