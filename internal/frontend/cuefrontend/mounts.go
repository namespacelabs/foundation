package cuefrontend

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

func ParseMounts(ctx context.Context, pl workspace.EarlyPackageLoader, loc workspace.Location, volumes []*schema.Volume, v *fncue.CueV) ([]*schema.Mount, []*schema.Volume, error) {
	it, err := v.Val.Fields()
	if err != nil {
		return nil, nil, err
	}

	inlinedVolumes := []*schema.Volume{}
	out := []*schema.Mount{}

	for it.Next() {
		volumeName, err := it.Value().String()
		if err == nil {
			// Volume reference.
			if findVolume(volumes, volumeName) == nil {
				return nil, nil, fnerrors.UserError(loc, "volume %q does not exist", volumeName)
			}
		} else {
			// Inline volume definition.
			volumeName := it.Label()
			if findVolume(volumes, volumeName) != nil {
				return nil, nil, fnerrors.UserError(loc, "volume %q already exists", volumeName)
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
