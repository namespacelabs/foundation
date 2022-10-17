// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"context"

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

	inlinedVolumes := []*schema.Volume{}
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
		} else {
			// Inline volume definition.
			volumeName := it.Label()

			parsedVolume, err := parseVolume(ctx, pl, loc, volumeName, true /* isInlined */, it.Value())
			if err != nil {
				return nil, nil, err
			}

			mount.VolumeRef = schema.MakePackageRef(loc.PackageName, volumeName)
			inlinedVolumes = append(inlinedVolumes, parsedVolume)
		}

		out = append(out, mount)
	}

	return out, inlinedVolumes, nil
}
