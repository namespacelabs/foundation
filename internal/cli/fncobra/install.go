// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fncobra

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func NewInstallCmd(binaryName string) *cobra.Command {
	var installDir string
	var quiet bool

	cmd := &cobra.Command{
		Use:   "install",
		Short: fmt.Sprintf("Install %s to local directory", binaryName),
	}

	cmd.Flags().StringVar(&installDir, "dir", "~/.local/bin", fmt.Sprintf("Directory to install %s to", binaryName))
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Suppress PATH warning output")

	cmd.RunE = RunE(func(ctx context.Context, args []string) error {
		execPath, err := os.Executable()
		if err != nil {
			return fnerrors.InternalError("failed to get executable path: %w", err)
		}

		binDir := installDir
		if strings.HasPrefix(binDir, "~/") {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fnerrors.InternalError("failed to get home directory: %w", err)
			}
			binDir = filepath.Join(homeDir, binDir[2:])
		}

		if err := os.MkdirAll(binDir, 0755); err != nil {
			return fnerrors.InternalError("failed to create directory %s: %w", binDir, err)
		}

		destPath := filepath.Join(binDir, binaryName)
		if err := copyFile(execPath, destPath); err != nil {
			return fnerrors.InternalError("failed to copy executable: %w", err)
		}

		if err := os.Chmod(destPath, 0755); err != nil {
			return fnerrors.InternalError("failed to make executable: %w", err)
		}

		fmt.Fprintf(console.Stdout(ctx), "✓ Installed %s to %s\n", binaryName, destPath)

		if !quiet {
			PrintPathWarning(ctx, binDir)
		}

		return nil
	})

	return cmd
}

func PrintPathWarning(ctx context.Context, binDir string) {
	path := os.Getenv("PATH")
	inPath := slices.Contains(strings.Split(path, ":"), binDir)

	if !inPath {
		shell := os.Getenv("SHELL")
		configFile := "~/.bashrc"
		if strings.Contains(shell, "zsh") {
			configFile = "~/.zshrc"
		}

		fmt.Fprintf(console.Stdout(ctx), "\n⚠ Setup notes:\n")
		fmt.Fprintf(console.Stdout(ctx), "  • %s is not in your PATH. Run: echo 'export PATH=\"%s:$PATH\"' >> %s && source %s\n", binDir, binDir, configFile, configFile)
	}
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
