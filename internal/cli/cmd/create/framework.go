// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package create

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

func selectFramework(ctx context.Context, title string, fmwkFlag string) (*schema.Framework, error) {
	frameworks := []frameworkItem{
		{schema.Framework_GO, "Go gRPC and HTTP handlers (beta)."},
	}

	var item frameworkItem
	if fmwkFlag != "" {
		for _, f := range frameworks {
			if strings.ToLower(f.Title()) == fmwkFlag {
				item = f
				break
			}
		}
		if item.fwmk == 0 {
			return nil, fnerrors.New("invalid framework: %s", fmwkFlag)
		}
	} else {
		selected, err := tui.Select(ctx, title, frameworks)
		if err != nil {
			return nil, err
		}

		if selected == nil {
			return nil, err
		}

		item = selected.(frameworkItem)
	}

	return &item.fwmk, nil
}

type frameworkItem struct {
	fwmk schema.Framework
	desc string
}

func (f frameworkItem) Title() string       { return f.fwmk.String() }
func (f frameworkItem) Description() string { return f.desc }
func (f frameworkItem) FilterValue() string { return f.Title() }

func withFramework(fmwkOut *string) *fmwkParser {
	return &fmwkParser{fmwkOut: fmwkOut}
}

type fmwkParser struct {
	fmwkOut *string
}

func (p *fmwkParser) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(p.fmwkOut, "framework", "", "The framework to use (go or web).")
}

func (p *fmwkParser) Parse(ctx context.Context, args []string) error { return nil }
