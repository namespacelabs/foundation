// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package binary

import (
	"context"
	"strings"

	"cuelang.org/go/cue"
	"golang.org/x/exp/maps"
	"k8s.io/utils/strings/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	integrationparsing "namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/api"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	integrationapplying "namespacelabs.dev/foundation/internal/parsing/integration/api"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type ParseImageOpts struct {
	Required bool
}

// Parses "image"/"imageFrom"/<string> fields.
// If needed, generates a binary with the given name and adds it to the package.
// Returns nil if neither of the fields is present.
func ParseImage(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, pkg *pkggraph.Package, binaryName string, v *fncue.CueV, opts ParseImageOpts) (*schema.PackageRef, error) {
	switch v.Val.Kind() {
	case cue.StringKind:
		ref, _ := v.Val.String()
		if ref == "" {
			break
		}

		return checkLoadRef(ctx, pl, pkg.Location, ref)

	case cue.StructKind:
		strct, _ := v.Val.Struct()
		fds := fields(strct, "image", "imageFrom")

		if len(fds) == 0 {
			break
		} else if len(fds) > 1 {
			return nil, fnerrors.NewWithLocation(pkg.Location, "expected one of <string> or a <struct> with exactly one field with label 'image' or 'imageFrom': got %s",
				strings.Join(maps.Keys(fds), ", "))
		}

		switch maps.Keys(fds)[0] {
		case "image":
			str, err := fds["image"].String()
			if err != nil {
				return nil, err
			}

			inlineBinary := &schema.Binary{
				Name: binaryName,
				BuildPlan: &schema.LayeredImageBuildPlan{
					LayerBuildPlan: []*schema.ImageBuildPlan{{ImageId: str}},
				},
			}

			outRef := schema.MakePackageRef(pkg.PackageName(), binaryName)
			pkg.Binaries = append(pkg.Binaries, inlineBinary)

			return outRef, nil

		case "imageFrom":
			x := &fncue.CueV{Val: fds["imageFrom"]}
			if binary := x.LookupPath("binary"); binary.Exists() {
				str, err := binary.Val.String()
				if err != nil {
					return nil, err
				}

				return checkLoadRef(ctx, pl, pkg.Location, str)
			} else {
				integration, err := integrationparsing.BuildParser.ParseEntity(ctx, env, pl, pkg.Location, x)
				if err != nil {
					return nil, err
				}

				return integrationapplying.GenerateBinaryAndAddToPackage(ctx, env, pl, pkg, binaryName, integration.Data)
			}

		default:
			return nil, fnerrors.NewWithLocation(pkg.Location, "expected one of <string> or a <struct> with exactly one field with label 'image' or 'imageFrom', got %q", maps.Keys(fds)[0])
		}

	case cue.BottomKind:
		break

	default:
		return nil, fnerrors.NewWithLocation(pkg.Location, "expected one of <string> or a <struct> with exactly one field with label 'image' or 'imageFrom': got %v", v.Val.Kind())
	}

	if opts.Required {
		return nil, fnerrors.NewWithLocation(pkg.Location, "required one of <string> or a <struct> with exactly one field with label 'image' or 'imageFrom'")
	}

	return nil, nil
}

type keyValue struct {
	Label string
	Value cue.Value
}

func fields(strct *cue.Struct, filter ...string) map[string]cue.Value {
	iter := strct.Fields()

	kvs := map[string]cue.Value{}
	for iter.Next() {
		key := iter.Selector().String()
		if len(filter) > 0 && !slices.Contains(filter, key) {
			continue
		}
		kvs[key] = iter.Value()
	}
	return kvs
}

func checkLoadRef(ctx context.Context, pl pkggraph.PackageLoader, owner pkggraph.Location, ref string) (*schema.PackageRef, error) {
	outRef, err := schema.ParsePackageRef(owner.PackageName, ref)
	if err != nil {
		return nil, fnerrors.NewWithLocation(owner, "parsing binary reference: %w", err)
	}

	if outRef.AsPackageName() != owner.PackageName {
		if _, err := pl.LoadByName(ctx, outRef.AsPackageName()); err != nil {
			return nil, err
		}
	}

	return outRef, nil
}
