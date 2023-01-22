// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"bytes"
	"context"
	"encoding/json"

	"cuelang.org/go/cue"
	"github.com/docker/go-units"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type cueBinary struct {
	Name      string                    `json:"name,omitempty"`
	Config    *schema.BinaryConfig      `json:"config,omitempty"`
	From      *cueImageBuildPlan        `json:"from,omitempty"`
	BuildPlan *cueLayeredImageBuildPlan `json:"build_plan,omitempty"`
	Labels    map[string]string         `json:"labels"`
}

type cueLayeredImageBuildPlan struct {
	LayerBuildPlan []*cueImageBuildPlan
}

var _ json.Unmarshaler = &cueLayeredImageBuildPlan{}

type cueImageBuildPlan struct {
	Prebuilt                 string                                 `json:"prebuilt,omitempty"`
	GoPackage                string                                 `json:"go_package,omitempty"`
	GoBuild                  *schema.ImageBuildPlan_GoBuild         `json:"go_build,omitempty"`
	Dockerfile               string                                 `json:"dockerfile,omitempty"`
	LlbPlan                  *cueImageBuildPlan_LLBPlan             `json:"llb_plan,omitempty"`
	NixFlake                 string                                 `json:"nix_flake,omitempty"`
	Deprecated_SnapshotFiles []string                               `json:"snapshot_files,omitempty"` // Use `files` instead.
	Files                    []string                               `json:"files,omitempty"`
	AlpineBuild              *schema.ImageBuildPlan_AlpineBuild     `json:"alpine_build,omitempty"`
	NodejsBuild              *schema.NodejsBuild                    `json:"nodejs_build,omitempty"`
	Binary                   string                                 `json:"binary,omitempty"`
	FilesFrom                *cueImageBuildPlan_FilesFrom           `json:"files_from,omitempty"`
	MakeSquashFS             *cueImageBuildPlan_MakeSquashFS        `json:"make_squashfs,omitempty"`
	MakeFilesystemImage      *cueImageBuildPlan_MakeFilesystemImage `json:"make_fs_image,omitempty"`
	ImageID                  string                                 `json:"image_id,omitempty"`
}

type cueImageBuildPlan_LLBPlan struct {
	OutputOf cueBinary `json:"output_of,omitempty"`
}

type cueImageBuildPlan_FilesFrom struct {
	From      cueImageBuildPlan `json:"from"`
	Files     []string          `json:"files"`
	TargetDir string            `json:"target_dir"`
}

type cueImageBuildPlan_MakeSquashFS struct {
	From   cueImageBuildPlan `json:"from"`
	Target string            `json:"target"`
}

type cueImageBuildPlan_MakeFilesystemImage struct {
	From   cueImageBuildPlan `json:"from"`
	Target string            `json:"target"`
	Kind   string            `json:"kind"`
	Size   string            `json:"size"`
}

func parseCueBinary(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, parent, v *fncue.CueV) (*schema.Binary, error) {
	// Ensure all fields are bound.
	if err := v.Val.Validate(cue.Concrete(true)); err != nil {
		return nil, err
	}

	var srcBin cueBinary
	if err := v.Val.Decode(&srcBin); err != nil {
		return nil, err
	}

	bin, err := srcBin.ToSchema(ctx, pl, loc)
	if err != nil {
		return nil, err
	}

	return bin, nil
}

func (srcBin cueBinary) ToSchema(ctx context.Context, pl parsing.EarlyPackageLoader, loc fnerrors.Location) (*schema.Binary, error) {
	bin := &schema.Binary{
		Name:   srcBin.Name,
		Config: srcBin.Config,
	}

	if srcBin.BuildPlan != nil {
		var err error
		bin.BuildPlan, err = srcBin.BuildPlan.ToSchema(ctx, pl, loc)
		if err != nil {
			return nil, err
		}
	}

	if srcBin.From != nil {
		if srcBin.BuildPlan != nil {
			return nil, fnerrors.NewWithLocation(loc, "from and build_plan are exclusive -- only one can be set")
		}

		parsed, err := srcBin.From.ToSchema(ctx, pl, loc)
		if err != nil {
			return nil, err
		}

		bin.BuildPlan = &schema.LayeredImageBuildPlan{
			LayerBuildPlan: []*schema.ImageBuildPlan{parsed},
		}
	}

	for k, v := range srcBin.Labels {
		bin.Labels = append(bin.Labels, &schema.Label{Name: k, Value: v})
	}

	bin.Labels = sortLabels(bin.Labels)

	return bin, nil
}

func (lbp *cueLayeredImageBuildPlan) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))

	tok, err := dec.Token()
	if err != nil {
		return err
	}

	switch tok {
	case json.Delim('['):
		return json.Unmarshal(data, &lbp.LayerBuildPlan)

	case json.Delim('{'):
		var x struct {
			LayerBuildPlan []*cueImageBuildPlan `json:"layer_build_plan,omitempty"`
		}
		if err := json.Unmarshal(data, &x); err != nil {
			return err
		}
		lbp.LayerBuildPlan = x.LayerBuildPlan
		return nil

	default:
		return fnerrors.BadInputError("unexpected input, expected array or object")
	}
}

func (lbp *cueLayeredImageBuildPlan) ToSchema(ctx context.Context, pl parsing.EarlyPackageLoader, loc fnerrors.Location) (*schema.LayeredImageBuildPlan, error) {
	plan := &schema.LayeredImageBuildPlan{}
	for _, def := range lbp.LayerBuildPlan {
		parsed, err := def.ToSchema(ctx, pl, loc)
		if err != nil {
			return nil, err
		}
		plan.LayerBuildPlan = append(plan.LayerBuildPlan, parsed)
	}
	return plan, nil
}

func (bp cueImageBuildPlan) ToSchema(ctx context.Context, pl parsing.EarlyPackageLoader, loc fnerrors.Location) (*schema.ImageBuildPlan, error) {
	plan := &schema.ImageBuildPlan{}

	var set []string
	if bp.Prebuilt != "" {
		plan.ImageId = bp.Prebuilt
		set = append(set, "prebuilt")
	}

	if bp.GoPackage != "" {
		plan.GoPackage = bp.GoPackage
		set = append(set, "go_package")
	}

	if bp.GoBuild != nil {
		plan.GoBuild = bp.GoBuild
		set = append(set, "go_build")
	}

	if bp.Dockerfile != "" {
		plan.Dockerfile = bp.Dockerfile
		set = append(set, "dockerfile")
	}

	if bp.NodejsBuild != nil {
		plan.NodejsBuild = bp.NodejsBuild
		set = append(set, "nodejs_build")
	}

	if bp.LlbPlan != nil {
		p, err := bp.LlbPlan.OutputOf.ToSchema(ctx, pl, loc)
		if err != nil {
			return nil, err
		}
		plan.LlbPlan = &schema.ImageBuildPlan_LLBPlan{OutputOf: p}
		set = append(set, "llb_plan")
	}

	if bp.NixFlake != "" {
		plan.NixFlake = bp.NixFlake
		set = append(set, "nix_flake")
	}

	if bp.Deprecated_SnapshotFiles != nil {
		plan.SnapshotFiles = bp.Deprecated_SnapshotFiles
		set = append(set, "snapshot_files")
	}

	if bp.Files != nil {
		plan.SnapshotFiles = bp.Files
		set = append(set, "files")
	}

	if bp.AlpineBuild != nil {
		plan.AlpineBuild = bp.AlpineBuild
		set = append(set, "alpine_build")
	}

	if bp.FilesFrom != nil {
		from, err := bp.FilesFrom.From.ToSchema(ctx, pl, loc)
		if err != nil {
			return nil, err
		}

		plan.FilesFrom = &schema.ImageBuildPlan_FilesFrom{
			From:      from,
			Files:     bp.FilesFrom.Files,
			TargetDir: bp.FilesFrom.TargetDir,
		}

		set = append(set, "files_from")
	}

	if bp.MakeSquashFS != nil {
		from, err := bp.MakeSquashFS.From.ToSchema(ctx, pl, loc)
		if err != nil {
			return nil, err
		}

		plan.MakeSquashfs = &schema.ImageBuildPlan_MakeSquashFS{
			From:   from,
			Target: bp.MakeSquashFS.Target,
		}

		set = append(set, "make_squashfs")
	}

	if bp.MakeFilesystemImage != nil {
		from, err := bp.MakeFilesystemImage.From.ToSchema(ctx, pl, loc)
		if err != nil {
			return nil, err
		}

		plan.MakeFsImage = &schema.ImageBuildPlan_MakeFilesystemImage{
			From:   from,
			Target: bp.MakeFilesystemImage.Target,
			Kind:   bp.MakeFilesystemImage.Kind,
		}

		if bp.MakeFilesystemImage.Size != "" {
			sizeBytes, err := units.FromHumanSize(bp.MakeFilesystemImage.Size)
			if err != nil {
				return nil, err
			}

			plan.MakeFsImage.Size = sizeBytes
		}

		set = append(set, "make_fs_image")
	}

	if bp.Binary != "" {
		ref, err := schema.StrictParsePackageRef(bp.Binary)
		if err != nil {
			return nil, err
		}

		if _, err := pl.LoadByName(ctx, ref.AsPackageName()); err != nil {
			return nil, err
		}

		plan.Binary = ref
		set = append(set, "binary")
	}

	if bp.ImageID != "" {
		plan.ImageId = bp.ImageID
		set = append(set, "image_id")
	}

	if len(set) == 0 {
		return nil, fnerrors.NewWithLocation(loc, "plan is missing at least one instruction")
	} else if len(set) > 1 {
		return nil, fnerrors.NewWithLocation(loc, "build plan must include exactly one instruction, saw %v", set)
	}

	return plan, nil
}
