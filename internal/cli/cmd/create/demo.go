// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"context"
	"fmt"

	"github.com/morikuni/aec"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnapi"
)

func newDemoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "demo",
		Short: "Creates an isolated demo environment.",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			userAuth, err := fnapi.LoadUser()
			if err != nil {
				return err
			}

			stdout := console.Stdout(ctx)
			fmt.Fprintf(stdout, "%s\n", aec.Bold.Apply("Namespace Demo"))

			name, err := tui.Ask(ctx, "What name should the new repository have?",
				"Namespace will create a demo repository under your Github account that contains an isolated development environment.\n",
				"namespace-demo")
			if err != nil {
				return err
			}

			access, err := tui.Select(ctx, "Which access type should the new repository have?", []accessItem{
				{private: false, desc: "Anyone on the internet can see this repository. You choose who can commit."},
				{private: true, desc: "Noone can see or commit to this repository."},
			})
			if err != nil {
				return err
			}

			url, err := fnapi.CreateWorkspace(ctx, userAuth, access.(accessItem).private, name)
			if err != nil {
				return fmt.Errorf("Unable to provision Namespace demo: %w", err)
			}

			if err = browser.OpenURL(url); err != nil {
				fmt.Fprintf(stdout, "Your demo is ready. Please open:\n\n  %s\n", url)
			} else {
				fmt.Fprintf(stdout, "Your demo is ready. Please continue in your browser.\n\n  %s\n", url)
			}

			return nil
		}),
	}

	return cmd
}

type accessItem struct {
	private bool
	desc    string
}

func (i accessItem) Title() string {
	if i.private {
		return "PRIVATE"
	}
	return "PUBLIC"
}
func (i accessItem) Description() string { return i.desc }
func (i accessItem) FilterValue() string { return i.Title() }
