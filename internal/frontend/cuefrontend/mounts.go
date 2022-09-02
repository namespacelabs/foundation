// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"context"
	"unicode"

	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

func ParseMounts(ctx context.Context, pl workspace.EarlyPackageLoader, loc workspace.Location, v *fncue.CueV) ([]*schema.Mount, []*schema.Volume, error) {
	it, err := v.Val.Fields()
	if err != nil {
		return nil, nil, err
	}

	inlinedVolumes := []*schema.Volume{}
	out := []*schema.Mount{}

	for it.Next() {
		volumeName, err := it.Value().String()
		if err != nil {
			// Inline volume definition. Generating a k8s-compatible volume name.
			volumeName = ""
			for _, c := range it.Label() {
				if unicode.IsLetter(c) || unicode.IsDigit(c) || c == '-' {
					volumeName += string(c)
				} else {
					volumeName += "-"
				}
			}

			parsedVolume, err := parseVolume(ctx, pl, loc, volumeName, true /* isInlined */, it.Value())
			if err != nil {
				return nil, nil, err
			}

			inlinedVolumes = append(inlinedVolumes, parsedVolume)
		}

		out = append(out, &schema.Mount{
			Owner:      loc.PackageName.String(),
			Path:       it.Label(),
			VolumeName: volumeName,
		})
	}

	return out, inlinedVolumes, nil
}
