// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package config

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema"
)

type DehydrateOpts struct {
	IncludeTextProto bool
}

func (opts DehydrateOpts) DehydrateTo(ctx context.Context, env *schema.Environment, stack *schema.Stack, ingress []*schema.IngressFragment, computed *schema.ComputedConfigurations, target fnfs.WriteFS) error {
	messages, err := protos.SerializeOpts{TextProto: opts.IncludeTextProto}.Serialize(
		env,
		stack,
		&schema.IngressFragmentList{IngressFragment: ingress},
		computed,
	)
	if err != nil {
		return err
	}

	senv := messages[0]
	sstack := messages[1]
	singress := messages[2]
	scomputedConfigs := messages[3]

	for _, f := range []fnfs.File{
		{Path: "config/env.binarypb", Contents: senv.Binary},
		{Path: "config/stack.binarypb", Contents: sstack.Binary},
		{Path: "config/ingress.binarypb", Contents: singress.Binary},
		{Path: "config/computed_configs.binarypb", Contents: scomputedConfigs.Binary},
	} {
		if err := fnfs.WriteFile(ctx, target, f.Path, f.Contents, 0644); err != nil {
			return err
		}
	}

	if opts.IncludeTextProto {
		for _, f := range []fnfs.File{
			{Path: "config/env.textpb", Contents: senv.Text},
			{Path: "config/stack.textpb", Contents: sstack.Text},
			{Path: "config/ingress.textpb", Contents: singress.Text},
			{Path: "config/computed_configs.textpb", Contents: scomputedConfigs.Text},
		} {
			if err := fnfs.WriteFile(ctx, target, f.Path, f.Contents, 0644); err != nil {
				return err
			}
		}
	}

	return nil
}
