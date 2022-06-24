// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

func selectFramework(ctx context.Context, title string, fmwkFlag *string) (*schema.Framework, error) {
	frameworks := []frameworkItem{
		{schema.Framework_GO, "Go gRPC and HTTP handlers (beta)."},
		{schema.Framework_WEB, "Typescript-based web application, built with Vite (alpha)."},
		{schema.Framework_NODEJS, "Typescript on Node.JS, gRPC and HTTP handlers (work in progress)."},
	}

	var item frameworkItem
	if *fmwkFlag != "" {
		for _, f := range frameworks {
			if strings.ToLower(f.Title()) == *fmwkFlag {
				item = f
				break
			}
		}
		if item.fwmk == 0 {
			return nil, fnerrors.UserError(nil, "invalid framework: %s", *fmwkFlag)
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

func frameworkFlag(cmd *cobra.Command) *string {
	return cmd.Flags().String("framework", "", "The framework to use (go, web or nodejs).")
}
