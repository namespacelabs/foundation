// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fncobra

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/protobuf/types/known/timestamppb"
	dur "namespacelabs.dev/foundation/internal/duration"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/module"
	"namespacelabs.dev/integrations/proto/namespace/stdlib"
)

func EnvFromValue(cmd *cobra.Command, env *string) *cfg.Context {
	target := new(cfg.Context)

	PushParse(cmd, func(ctx context.Context, args []string) error {
		root, err := module.FindRoot(ctx, ".")
		if err != nil {
			return err
		}

		sourceEnv := *env
		if sourceEnv == "" {
			sourceEnv = "dev"
		}

		env, err := cfg.LoadContext(root, sourceEnv)
		if err != nil {
			return err
		}

		*target = env
		return nil
	})

	return target
}

func LocationsFromArgs(cmd *cobra.Command, env *cfg.Context, opts ...ParseLocationsOpts) *Locations {
	target := new(Locations)

	PushParse(cmd, func(ctx context.Context, args []string) error {
		locations, err := ParseLocs(ctx, args, env, MergeParseLocationOpts(opts))
		if err != nil {
			return err
		}
		*target = *locations
		return nil
	})

	return target
}

func PushParse(cmd *cobra.Command, handler func(ctx context.Context, args []string) error) {
	previous := cmd.PreRunE
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if previous != nil {
			if err := previous(cmd, args); err != nil {
				return err
			}
		}

		return handler(cmd.Context(), args)
	}
}

func PushPreParse(cmd *cobra.Command, handler func(ctx context.Context, args []string) error) {
	previous := cmd.PersistentPreRunE
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if previous != nil {
			if err := previous(cmd, args); err != nil {
				return err
			}
		}

		return handler(cmd.Context(), args)
	}
}

// Custom duration flag type to support parsing extra units: "d" (day) and "w" (week).
type duration struct {
	value *time.Duration
}

func (d *duration) Set(s string) error {
	parsed, err := dur.ParseDuration(s)
	if err != nil {
		return err
	}
	*d.value = parsed
	return nil
}

func (d *duration) String() string {
	return d.value.String()
}

func (d *duration) Type() string {
	return "duration"
}

func DurationVar(flags *pflag.FlagSet, d *time.Duration, name string, value time.Duration, usage string) {
	wrapped := duration{d}
	*wrapped.value = value

	flags.Var(&wrapped, name, usage)
}

func Duration(flags *pflag.FlagSet, name string, value time.Duration, usage string) *time.Duration {
	var res time.Duration

	DurationVar(flags, &res, name, value, usage)

	return &res
}

// ParseTimestampRange parses exclusive RFC3339 bounds. Nil or empty inputs are unbounded.
func ParseTimestampRange(before, after *string) (*stdlib.TimestampRange, error) {
	parse := func(bound string, value *string) (*timestamppb.Timestamp, error) {
		if value == nil || *value == "" {
			return nil, nil
		}
		t, err := time.Parse(time.RFC3339, *value)
		if err != nil {
			return nil, fnerrors.BadInputError("invalid %s timestamp: %w", bound, err)
		}
		ts := timestamppb.New(t.UTC())
		if err := ts.CheckValid(); err != nil {
			return nil, fnerrors.BadInputError("invalid %s timestamp: %w", bound, err)
		}
		return ts, nil
	}
	a, err := parse("after", after)
	if err != nil {
		return nil, err
	}
	b, err := parse("before", before)
	if err != nil {
		return nil, err
	}
	if a == nil && b == nil {
		return nil, nil
	}
	if a != nil && b != nil && !a.AsTime().Before(b.AsTime()) {
		return nil, fnerrors.BadInputError("after must be earlier than before")
	}
	return &stdlib.TimestampRange{After: a, Before: b}, nil
}
