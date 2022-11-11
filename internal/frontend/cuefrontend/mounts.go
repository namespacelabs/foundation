// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func ParseMounts(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, v *fncue.CueV) ([]*schema.Mount, []*schema.Volume, error) {
	it, err := v.Val.Fields()
	if err != nil {
		return nil, nil, err
	}

	volumes := []*schema.Volume{}
	out := []*schema.Mount{}

	for it.Next() {
		mount := &schema.Mount{
			Owner: loc.PackageName.String(),
			Path:  it.Label(),
		}
		if volumeRef, err := it.Value().String(); err == nil {
			mount.VolumeRef, err = schema.ParsePackageRef(loc.PackageName, volumeRef)
			if err != nil {
				return nil, nil, err
			}

			vol, err := loadVolume(ctx, pl, loc, mount.VolumeRef)
			if err != nil {
				return nil, nil, err
			}
			if vol != nil {
				volumes = append(volumes, vol)
			}
		} else {
			// Inline volume definition.
			volumeName := it.Label()

			vol, err := parseVolume(ctx, pl, loc, volumeName, true /* isInlined */, it.Value())
			if err != nil {
				return nil, nil, err
			}

			mount.VolumeRef = schema.MakePackageRef(loc.PackageName, volumeName)
			volumes = append(volumes, vol)
		}

		out = append(out, mount)
	}

	return out, volumes, nil
}

func loadVolume(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, ref *schema.PackageRef) (*schema.Volume, error) {
	if ref.Name == "" {
		return nil, fnerrors.NewWithLocation(loc, "volumes refs require a name: got %q, expected \"pkg:foo\" or \":foo\"", ref.Canonical())
	}

	pkg := ref.AsPackageName()
	if loc.PackageName == pkg {
		// All volumes from the same package are already added by default.
		return nil, nil
	}

	loaded, err := pl.LoadByName(ctx, pkg)
	if err != nil {
		return nil, err
	}
	for _, v := range loaded.Volumes {
		if v.Name == ref.Name {
			return v, nil
		}
	}

	// TODO consolidate error UX.
	return nil, fnerrors.NewWithLocation(loc, "No volume %q found in package %s", ref.Name, ref.PackageName)
}
