// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"context"
	"io"
	"io/fs"
	"strings"

	"cuelang.org/go/cue"
	"github.com/docker/go-units"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/runtime/constants"
	"namespacelabs.dev/foundation/workspace"
)

func ParseVolumes(ctx context.Context, pl workspace.EarlyPackageLoader, loc pkggraph.Location, v *fncue.CueV) ([]*schema.Volume, error) {
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
	FromDir              string `json:"fromDir"`
	FromFile             string `json:"fromFile"`
	FromSecret           string `json:"fromSecret"`
	FromKubernetesSecret string `json:"fromKubernetesSecret"`
}

func parseVolume(ctx context.Context, pl workspace.EarlyPackageLoader, loc pkggraph.Location, name string, isInlined bool, value cue.Value) (*schema.Volume, error) {
	var bits cueVolume
	if err := value.Decode(&bits); err != nil {
		return nil, err
	}

	// Parsing shortcuts
	if bits.Kind == "" {
		if bits.Ephemeral != nil {
			bits.Kind = constants.VolumeKindEphemeral
		}
		if bits.Persistent != nil {
			bits.cuePersistentVolume = *bits.Persistent
			bits.Kind = constants.VolumeKindPersistent
		}
		if bits.PackageSync != nil {
			bits.cueFilesetVolume = *bits.PackageSync
			bits.Kind = constants.VolumeKindPackageSync
		}
		if bits.Configurable != nil {
			// Parsing can't be done via JSON unmarshalling, so doing it manually below.
			bits.Kind = constants.VolumeKindConfigurable
		}
	}

	out := &schema.Volume{
		Owner:  loc.PackageName.String(),
		Kind:   bits.Kind,
		Name:   name,
		Inline: isInlined,
	}

	var definition proto.Message

	switch bits.Kind {
	case constants.VolumeKindEphemeral:
		definition = &schema.EphemeralVolume{}

	case constants.VolumeKindPersistent:
		sizeBytes, err := units.FromHumanSize(bits.Size)
		if err != nil {
			return nil, fnerrors.Wrapf(loc, err, "failed to parse value")
		}
		definition = &schema.PersistentVolume{
			Id:        bits.Id,
			SizeBytes: uint64(sizeBytes),
		}

	case constants.VolumeKindConfigurable:
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
				parsedEntry, err := parseConfigurableEntry(ctx, pl, loc, isInlined, it.Value())
				if err != nil {
					return nil, fnerrors.Wrapf(loc, err, it.Label())
				}
				parsedEntry.Path = it.Label()
				entries = append(entries, parsedEntry)
			}
		} else {
			// Entry name is the volume/mount name.
			entry, err := parseConfigurableEntry(ctx, pl, loc, isInlined, val.Val)
			if err != nil {
				return nil, fnerrors.Wrapf(loc, err, name)
			}
			entry.Path = name
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

func parseConfigurableEntry(ctx context.Context, pl workspace.EarlyPackageLoader, loc pkggraph.Location, isVolumeInlined bool, v cue.Value) (*schema.ConfigurableVolume_Entry, error) {
	if v.Kind() == cue.StringKind {
		// Inlined content.
		str, _ := v.String()
		if !isVolumeInlined {
			return nil, fnerrors.UserError(loc, "Configurable content %q without target path must be inlined", str)
		}

		return &schema.ConfigurableVolume_Entry{
			Inline: &schema.FileContents{
				Contents: []byte(str),
				Utf8:     true,
			},
		}, nil
	}

	var bits cueConfigurableEntry
	if err := v.Decode(&bits); err != nil {
		return nil, err
	}

	fsys, err := pl.WorkspaceOf(ctx, loc.Module)
	if err != nil {
		return nil, err
	}

	switch {
	case bits.FromDir != "":
		snapshot, err := memfs.SnapshotDir(fsys, bits.FromDir, memfs.SnapshotOpts{})
		if err != nil {
			return nil, fnerrors.InternalError("%s: failed to snapshot: %w", bits.FromDir, err)
		}

		set := &schema.ResourceSet{}
		if err := fnfs.VisitFiles(ctx, snapshot, func(path string, bs bytestream.ByteStream, de fs.DirEntry) error {
			r, err := bs.Reader()
			if err != nil {
				return err
			}

			contents, err := io.ReadAll(r)
			if err != nil {
				return fnerrors.InternalError("%s: failed to read contents: %w", path, err)
			}

			set.Resource = append(set.Resource, &schema.FileContents{
				Path:     loc.Rel(path),
				Contents: contents,
			})

			return nil
		}); err != nil {
			return nil, fnerrors.InternalError("%s: failed to consume snapshot: %w", bits.FromDir, err)
		}

		return &schema.ConfigurableVolume_Entry{InlineSet: set}, nil

	case bits.FromFile != "":
		rsc, err := LoadResource(fsys, loc, bits.FromFile)
		if err != nil {
			return nil, err
		}

		return &schema.ConfigurableVolume_Entry{Inline: rsc}, nil

	case bits.FromSecret != "":
		var secretRef *schema.PackageRef
		if strings.Contains(bits.FromSecret, ":") {
			secretRef, err = schema.ParsePackageRef(bits.FromSecret)
			if err != nil {
				return nil, err
			}
		} else {
			secretRef = schema.MakePackageRef(loc.PackageName, bits.FromSecret)
		}
		return &schema.ConfigurableVolume_Entry{SecretRef: secretRef}, nil

	case bits.FromKubernetesSecret != "":
		parts := strings.SplitN(bits.FromKubernetesSecret, ":", 2)
		if len(parts) != 2 {
			return nil, fnerrors.BadInputError("kubernetes secrets are specified as {name}:{key}")
		}

		return &schema.ConfigurableVolume_Entry{KubernetesSecretRef: &schema.ConfigurableVolume_Entry_KubernetesSecretRef{
			SecretName: parts[0],
			SecretKey:  parts[1],
		}}, nil

	default:
		return nil, fnerrors.UserError(loc, "must have a source")
	}
}
