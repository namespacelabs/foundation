// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/reflow/wordwrap"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
)

func NewDeploymentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "deployment",
	}

	envRef := "dev"
	defaultYes := false
	wait := true

	remove := &cobra.Command{
		Use:   "remove",
		Short: "Removes all deployment assets associated with the specified environment.",
		Args:  cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			env, err := requireEnv(ctx, envRef)
			if err != nil {
				return err
			}

			if !defaultYes {
				if err := checkDelete(ctx, envRef); err != nil {
					return err
				}
			}

			removed, err := runtime.For(ctx, env).DeleteRecursively(ctx, wait)
			if removed {
				fmt.Fprintln(console.Stdout(ctx), "Resources removed.")
			} else if err == nil {
				fmt.Fprintln(console.Stdout(ctx), "Nothing to remove.")
			}

			return err
		}),
	}

	remove.Flags().StringVar(&envRef, "env", envRef, "Specifies the environment to apply to.")
	remove.Flags().BoolVar(&defaultYes, "yes", defaultYes, "If set to true, assume yes on prompts.")
	remove.Flags().BoolVar(&wait, "wait", wait, "If set to true, waits until all resources are removed before returning.")

	cmd.AddCommand(remove)

	return cmd
}

func checkDelete(ctx context.Context, env string) error {
	done := console.EnterInputMode(ctx)
	defer done()

	p := tea.NewProgram(initialModel(env))

	final, err := p.StartReturningModel()
	if err != nil {
		return err
	}

	if final.(envInputModel).canceled {
		return context.Canceled
	}

	if final.(envInputModel).textInput.Value() != env {
		return fnerrors.New("environment name didn't match, canceling")
	}

	return nil
}

type envInputModel struct {
	env       string
	textInput textinput.Model
	canceled  bool
}

func initialModel(env string) envInputModel {
	ti := textinput.New()
	ti.Placeholder = env
	ti.Focus()
	ti.CharLimit = 32
	ti.Width = 32

	return envInputModel{
		env:       env,
		textInput: ti,
	}
}

func (m envInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m envInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			return m, tea.Quit

		case tea.KeyCtrlC, tea.KeyEsc:
			m.canceled = true
			return m, tea.Quit
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m envInputModel) View() string {
	return wordwrap.String(fmt.Sprintf(
		"Removing a deployment is a destructive operation -- any data that is a part of the environment will not be recoverable.\n\nPlease type %q to confirm you'd like to remove all of its resources.\n\n%s\n\n%s",
		m.env,
		m.textInput.View(),
		"(esc to quit)",
	)+"\n", 80)
}
