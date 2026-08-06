// Copyright 2026 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

func NewCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "completion",
		Short:  "Generate shell completion scripts",
		Hidden: true,
		Long: `Generate shell completion scripts for nsc.

To load completions in your current shell session:

  Bash:
    source <(nsc completion bash)

  Fish:
    nsc completion fish | source

  Zsh:
    source <(nsc completion zsh)

  PowerShell:
    nsc completion powershell | Out-String | Invoke-Expression

To install completions permanently:

  Bash:
    nsc completion bash > /etc/bash_completion.d/nsc
    # or, for a single user:
    nsc completion bash > ~/.local/share/bash-completion/completions/nsc

  Fish:
    nsc completion fish > ~/.config/fish/completions/nsc.fish

  Zsh:
    nsc completion zsh > "${fpath[1]}/_nsc"

  PowerShell:
    nsc completion powershell > $PROFILE.CurrentUserAllHosts
`,
		// Override the root's PersistentPreRunE to prevent console setup,
		// version checks, and other bootstrapping from contaminating stdout.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
	}

	cmd.AddCommand(newBashCompletionCmd())
	cmd.AddCommand(newFishCompletionCmd())
	cmd.AddCommand(newZshCompletionCmd())
	cmd.AddCommand(newPowerShellCompletionCmd())

	return cmd
}

func newBashCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "bash",
		Short:             "Generate bash completion script",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Root().GenBashCompletion(os.Stdout)
		},
	}
}

func newFishCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "fish",
		Short:             "Generate fish completion script",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		},
	}
}

func newZshCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "zsh",
		Short:             "Generate zsh completion script",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Root().GenZshCompletion(os.Stdout)
		},
	}
}

func newPowerShellCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "powershell",
		Short:             "Generate PowerShell completion script",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		},
	}
}
