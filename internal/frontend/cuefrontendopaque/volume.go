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
	volumeKindEphemeral    = "namespace.so/volume/ephemeral"
	volumeKindPersistent   = "namespace.so/volume/persistent"
	volumeKindConfigurable = "namespace.so/volume/configurable"
	volumeKindPackageSync  = "namespace.so/volume/package-sync"
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
		parsedVolume, err := parseVolume(ctx, loc, it.Label(), false /* isInlined */, it.Value())
		if err != nil {
			return nil, err
		}

		out = append(out, parsedVolume)
	}

	return out, nil
}

type cueVolume struct {
	cuePersistentVolume
	cueFilesetVolume

	Kind string `json:"kind"`

	// Shortcuts
	Ephemeral    interface{}          `json:"ephemeral"`
	Persistent   *cuePersistentVolume `json:"persistent"`
	PackageSync  *cueFilesetVolume    `json:"packageSync"`
	Configurable interface{}          `json:"configurable"`
}

type cuePersistentVolume struct {
	Id   string `json:"id"`
	Size string `json:"size"`
}

type cueFilesetVolume struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
}

type cueConfigurableEntry struct {
	FromDir    string `json:"fromDir"`
	FromFile   string `json:"fromFile"`
	FromSecret string `json:"fromSecret"`
}

func parseVolume(ctx context.Context, loc workspace.Location, name string, isInlined bool, value cue.Value) (*schema.Volume, error) {
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
		if bits.Configurable != nil {
			// Parsing can't be done via JSON unmarshalling, so doing it manually below.
			bits.Kind = volumeKindConfigurable
		}
	}

	out := &schema.Volume{
		Kind: bits.Kind,
		Name: name,
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
	case volumeKindConfigurable:
		val := &fncue.CueV{Val: value}
		if bits.Configurable != nil {
			cueV := fncue.CueV{Val: value}
			val = cueV.LookupPath("configurable")
		}

		entries := []*schema.Volume_Configurable_Entry{}
		contents := val.LookupPath("contents")
		if contents.Exists() {
			it, err := contents.Val.Fields()
			if err != nil {
				return nil, err
			}

			for it.Next() {
				parsedEntry, err := parseConfigurableEntry(ctx, loc, it.Label(), isInlined, it.Value())
				if err != nil {
					return nil, err
				}
				entries = append(entries, parsedEntry)
			}
		} else {
			// Entry name is the volume/mount name.
			entry, err := parseConfigurableEntry(ctx, loc, name, isInlined, val.Val)
			if err != nil {
				return nil, err
			}
			entries = append(entries, entry)
		}

		out.Configurable = &schema.Volume_Configurable{
			Entries: entries,
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

func parseConfigurableEntry(ctx context.Context, loc workspace.Location, name string, isVolumeInlined bool, v cue.Value) (*schema.Volume_Configurable_Entry, error) {
	if v.Kind() == cue.StringKind {
		// Inlined content
		strVal, _ := v.String()
		if !isVolumeInlined {
			return nil, fnerrors.UserError(loc, "Configurable content %q without target path must be inlined", strVal)
		}
		return &schema.Volume_Configurable_Entry{
			SourceInlinedContent: strVal,
		}, nil
	} else {
		var bits cueConfigurableEntry
		if err := v.Decode(&bits); err != nil {
			return nil, err
		}

		if bits.FromDir != "" {
			return &schema.Volume_Configurable_Entry{
				SourceDir: bits.FromDir,
			}, nil
		} else if bits.FromFile != "" {
			return &schema.Volume_Configurable_Entry{
				SourceFilePath: bits.FromFile,
			}, nil
		} else if bits.FromSecret != "" {
			// TODO: verify that a secret exists with the given name
			return &schema.Volume_Configurable_Entry{
				SourceSecret: bits.FromSecret,
			}, nil
		} else {
			return nil, fnerrors.UserError(loc, "Configurable entry %q must have a source", name)
		}
	}
}

func findVolume(name string, volumes []*schema.Volume) *schema.Volume {
	for _, v := range volumes {
		if v.Name == name {
			return v
		}
	}
	return nil
}
