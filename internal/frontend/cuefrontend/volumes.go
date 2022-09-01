// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"context"

	"cuelang.org/go/cue"
	"github.com/docker/go-units"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

func ParseVolumes(ctx context.Context, pl workspace.EarlyPackageLoader, loc workspace.Location, v *fncue.CueV) ([]*schema.Volume, error) {
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
		parsedVolume, err := parseVolume(ctx, pl, loc, it.Label(), false /* isInlined */, it.Value())
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

func parseVolume(ctx context.Context, pl workspace.EarlyPackageLoader, loc workspace.Location, name string, isInlined bool, value cue.Value) (*schema.Volume, error) {
	var bits cueVolume
	if err := value.Decode(&bits); err != nil {
		return nil, err
	}

	// Parsing shortcuts
	if bits.Kind == "" {
		if bits.Ephemeral != nil {
			bits.Kind = runtime.VolumeKindEphemeral
		}
		if bits.Persistent != nil {
			bits.cuePersistentVolume = *bits.Persistent
			bits.Kind = runtime.VolumeKindPersistent
		}
		if bits.PackageSync != nil {
			bits.cueFilesetVolume = *bits.PackageSync
			bits.Kind = runtime.VolumeKindPackageSync
		}
		if bits.Configurable != nil {
			// Parsing can't be done via JSON unmarshalling, so doing it manually below.
			bits.Kind = runtime.VolumeKindConfigurable
		}
	}

	out := &schema.Volume{
		Owner: loc.PackageName.String(),
		Kind:  bits.Kind,
		Name:  name,
	}

	var definition proto.Message

	switch bits.Kind {
	case runtime.VolumeKindEphemeral:
		definition = &schema.EphemeralVolume{}

	case runtime.VolumeKindPersistent:
		sizeBytes, err := units.FromHumanSize(bits.Size)
		if err != nil {
			return nil, fnerrors.Wrapf(loc, err, "failed to parse value")
		}
		definition = &schema.PersistentVolume{
			Id:        bits.Id,
			SizeBytes: uint64(sizeBytes),
		}

	case runtime.VolumeKindConfigurable:
		val := &fncue.CueV{Val: value}
		if bits.Configurable != nil {
			cueV := fncue.CueV{Val: value}
			val = cueV.LookupPath("configurable")
		}

		entries := []*schema.ConfigurableVolume_Entry{}
		contents := val.LookupPath("contents")
		if contents.Exists() {
			it, err := contents.Val.Fields()
			if err != nil {
				return nil, err
			}

			for it.Next() {
				parsedEntry, err := parseConfigurableEntry(ctx, pl, loc, it.Label(), isInlined, it.Value())
				if err != nil {
					return nil, err
				}
				entries = append(entries, parsedEntry)
			}
		} else {
			// Entry name is the volume/mount name.
			entry, err := parseConfigurableEntry(ctx, pl, loc, name, isInlined, val.Val)
			if err != nil {
				return nil, err
			}
			entries = append(entries, entry)
		}

		definition = &schema.ConfigurableVolume{
			Entries: entries,
		}

	default:
		return nil, fnerrors.BadInputError("%s: unsupported volume type", bits.Kind)
	}

	var err error
	out.Definition, err = anypb.New(definition)
	if err != nil {
		return nil, fnerrors.InternalError("failed to serialize definition: %w", err)
	}

	return out, nil
}

func parseConfigurableEntry(ctx context.Context, pl workspace.EarlyPackageLoader, loc workspace.Location, name string, isVolumeInlined bool, v cue.Value) (*schema.ConfigurableVolume_Entry, error) {
	fsys, err := pl.WorkspaceOf(ctx, loc.Module)
	if err != nil {
		return nil, err
	}

	if v.Kind() == cue.StringKind {
		// Inlined content.
		str, _ := v.String()
		if !isVolumeInlined {
			return nil, fnerrors.UserError(loc, "Configurable content %q without target path must be inlined", str)
		}
		return &schema.ConfigurableVolume_Entry{
			Inline: &schema.Resource{
				Contents: []byte(str),
			},
		}, nil
	} else {
		var bits cueConfigurableEntry
		if err := v.Decode(&bits); err != nil {
			return nil, err
		}

		switch {
		case bits.FromDir != "":
			return nil, fnerrors.InternalError("loading directory not implemented yet")

		case bits.FromFile != "":
			rsc, err := LoadResource(fsys, loc, bits.FromFile)
			if err != nil {
				return nil, err
			}

			return &schema.ConfigurableVolume_Entry{
				Inline: rsc,
			}, nil

		case bits.FromSecret != "":
			// TODO: verify that a secret exists with the given name
			return &schema.ConfigurableVolume_Entry{
				SecretRef: bits.FromSecret,
			}, nil

		default:
			return nil, fnerrors.UserError(loc, "configurable entry %q must have a source", name)
		}
	}
}

func findVolume(volumes []*schema.Volume, name string) *schema.Volume {
	for _, v := range volumes {
		if v.Name == name {
			return v
		}
	}
	return nil
}
