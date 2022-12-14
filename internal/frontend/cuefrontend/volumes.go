// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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
	"namespacelabs.dev/foundation/internal/hotreload"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/runtime/constants"
)

func ParseVolumes(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, v *fncue.CueV) ([]*schema.Volume, error) {
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
	cueWorkspaceSyncVolume

	Kind string `json:"kind"`

	// Shortcuts
	Ephemeral     any                     `json:"ephemeral,omitempty"`
	Persistent    *cuePersistentVolume    `json:"persistent,omitempty"`
	WorkspaceSync *cueWorkspaceSyncVolume `json:"syncWorkspace,omitempty"`
	Configurable  any                     `json:"configurable,omitempty"`
	HostPath      *cueHostPathVolume      `json:"hostPath,omitempty"`
}

type cuePersistentVolume struct {
	Id   string `json:"id"`
	Size string `json:"size"`
}

type cueWorkspaceSyncVolume struct {
	FromDir string `json:"fromDir"`
}

type cueHostPathVolume struct {
	Directory string `json:"directory"`
}

type cueConfigurableEntry struct {
	FromDir              string `json:"fromDir"`
	FromFile             string `json:"fromFile"`
	FromSecret           string `json:"fromSecret"`
	FromKubernetesSecret string `json:"fromKubernetesSecret"`
}

func parseVolume(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, name string, isInlined bool, value cue.Value) (*schema.Volume, error) {
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
		if bits.WorkspaceSync != nil {
			bits.cueWorkspaceSyncVolume = *bits.WorkspaceSync
			bits.Kind = constants.VolumeKindWorkspaceSync
		}
		if bits.Configurable != nil {
			// Parsing can't be done via JSON unmarshalling, so doing it manually below.
			bits.Kind = constants.VolumeKindConfigurable
		}
		if bits.HostPath != nil {
			bits.Kind = constants.VolumeKindHostPath
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
			return nil, fnerrors.NewWithLocation(loc, "failed to parse value: %w", err)
		}
		definition = &schema.PersistentVolume{
			Id:        bits.Id,
			SizeBytes: uint64(sizeBytes),
		}

	case constants.VolumeKindHostPath:
		if bits.HostPath == nil || bits.HostPath.Directory == "" {
			return nil, fnerrors.NewWithLocation(loc, "host: missing required field 'directory'")
		}

		definition = &schema.HostVolume{Directory: bits.HostPath.Directory}

	case constants.VolumeKindWorkspaceSync:
		if bits.FromDir == "" {
			return nil, fnerrors.NewWithLocation(loc, "workspace sync: missing required field 'fromDir'")
		}

		definition = &schema.WorkspaceSyncVolume{
			Path: bits.FromDir,
		}

		// Making sure that the controller package is loaded.
		_, err := pl.LoadByName(ctx, hotreload.ControllerPkg.AsPackageName())
		if err != nil {
			return nil, err
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
					return nil, fnerrors.NewWithLocation(loc, "%s: %w", it.Label(), err)
				}
				parsedEntry.Path = it.Label()
				entries = append(entries, parsedEntry)
			}
		} else {
			// Entry name is the volume/mount name.
			entry, err := parseConfigurableEntry(ctx, pl, loc, isInlined, val.Val)
			if err != nil {
				return nil, fnerrors.NewWithLocation(loc, "%s: %w", name, err)
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

func parseConfigurableEntry(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, isVolumeInlined bool, v cue.Value) (*schema.ConfigurableVolume_Entry, error) {
	if v.Kind() == cue.StringKind {
		// Inlined content.
		str, _ := v.String()
		if !isVolumeInlined {
			return nil, fnerrors.NewWithLocation(loc, "Configurable content %q without target path must be inlined", str)
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
		secretRef, err := schema.ParsePackageRef(loc.PackageName, bits.FromSecret)
		if err != nil {
			return nil, err
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
		return nil, fnerrors.NewWithLocation(loc, "must have a source")
	}
}
