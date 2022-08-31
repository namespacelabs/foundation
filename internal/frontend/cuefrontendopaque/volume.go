// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"
	"encoding/json"

	"cuelang.org/go/cue"
	"github.com/docker/go-units"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

const (
	volumeKindEphemeral  = "namespace.so/volume/ephemeral"
	volumeKindPersistent = "namespace.so/volume/persistent"
	// volumeKindConfigurable = "namespace.so/volume/configurable"
	volumeKindPackageSync = "namespace.so/volume/package-sync"
)

func parseVolumes(ctx context.Context, loc workspace.Location, v *fncue.CueV) ([]*schema.Volume, error) {
	// Ensure all fields are bound.
	if err := v.Val.Validate(cue.Concrete(true)); err != nil {
		return nil, err
	}

	it, err := v.Val.Fields()
	if err != nil {
		return nil, err
	}

	out := []*schema.Volume{}

	for it.Next() {
		parsedVolume, err := parseVolume(ctx, loc, it.Value())
		if err != nil {
			return nil, err
		}

		parsedVolume.Name = it.Label()

		out = append(out, parsedVolume)
	}

	return out, nil
}

type cueVolume struct {
	cuePersistentVolume
	cueFilesetVolume

	Kind string `json:"kind"`

	// Shortcuts
	Ephemeral   map[string]interface{} `json:"ephemeral"`
	Persistent  *cuePersistentVolume   `json:"persistent"`
	PackageSync *cueFilesetVolume      `json:"packageSync"`
}

type cuePersistentVolume struct {
	Id   string `json:"id"`
	Size string `json:"size"`
}

type cueFilesetVolume struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
}

func parseVolume(ctx context.Context, loc workspace.Location, value cue.Value) (*schema.Volume, error) {
	var bits cueVolume
	if err := value.Decode(&bits); err != nil {
		return nil, err
	}

	// Parsing shortcuts
	if bits.Kind == "" {
		if bits.Ephemeral != nil {
			bits.Kind = volumeKindEphemeral
		}
		if bits.Persistent != nil {
			bits.cuePersistentVolume = *bits.Persistent
			bits.Kind = volumeKindPersistent
		}
		if bits.PackageSync != nil {
			bits.cueFilesetVolume = *bits.PackageSync
			bits.Kind = volumeKindPackageSync
		}
	}

	out := &schema.Volume{
		Kind: bits.Kind,
	}

	switch bits.Kind {
	case volumeKindEphemeral:
		out.Ephemeral = &schema.Volume_Ephemeral{}
	case volumeKindPersistent:
		sizeBytes, err := units.RAMInBytes(bits.Size)
		if err != nil {
			return nil, fnerrors.Wrapf(loc, err, "failed to parse value")
		}
		out.Persistent = &schema.Volume_Persistent{
			Id:        bits.Id,
			SizeBytes: sizeBytes,
		}
	case volumeKindPackageSync:
		out.PackageSync = &schema.Volume_Fileset{
			IncludeFilePatterns: bits.Include,
			ExcludeFilePatterns: bits.Exclude,
		}
	default:
		var jsn map[string]interface{}
		if err := value.Decode(&jsn); err != nil {
			return nil, err
		}
		jsonStr, err := json.Marshal(jsn)
		if err != nil {
			return nil, err
		}
		out.Custom = string(jsonStr)
	}
	return out, nil
}

func findVolume(name string, volumes []*schema.Volume) *schema.Volume {
	for _, v := range volumes {
		if v.Name == name {
			return v
		}
	}
	return nil
}
