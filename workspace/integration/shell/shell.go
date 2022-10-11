// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package shell

import (
	"context"
	"io/fs"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func CreateBinary(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, loc pkggraph.Location, data *schema.ShellIntegration) (*schema.Binary, error) {
	if data.Script == "" {
		return nil, fnerrors.UserError(loc, "missing required field `script`")
	}

	if _, err := fs.Stat(loc.Module.ReadOnlyFS(), filepath.Join(loc.Rel(), data.Script)); err != nil {
		return nil, fnerrors.Wrapf(loc, err, "could not find %q file, please verify that the specified script path is correct", data.Script)
	}

	return &schema.Binary{
		BuildPlan: &schema.LayeredImageBuildPlan{
			LayerBuildPlan: []*schema.ImageBuildPlan{{
				ShellBuild: &schema.ImageBuildPlan_ShellBuild{
					Script:  data.Script,
					Install: data.Install,
				},
			}},
		},
		Config: &schema.BinaryConfig{
			Command: []string{data.Script},
		},
	}, nil
}
