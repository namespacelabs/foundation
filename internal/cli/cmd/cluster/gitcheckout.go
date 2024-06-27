// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
)

type contextKey string

var (
	_indentKey = contextKey("fn.nsc.git-checkout.indent")
)

type submodule struct {
	// Name of the entry in .gitmodules
	configKey string
	// Relative path where the submodule is checked out in the repo
	relativePath string
	// Remote repository url
	remoteUrl string
}

func NewGitCheckoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "git-checkout",
		Short:  "Actions for git checkouts supporting Namespace Cache Volumes.",
		Hidden: true,
	}

	mirrorBaseDir := cmd.PersistentFlags().String("mirror_base_path", "", "the path of the mirror base directory")
	cmd.MarkPersistentFlagRequired("mirror_base_path")

	cmd.AddCommand(newUpdateSubmodulesCmd(mirrorBaseDir))

	return cmd
}

func newUpdateSubmodulesCmd(mirrorBaseDir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-submodules",
		Short: "Updates git submodules using a Namespace git mirror.",
	}

	repositoryPath := cmd.Flags().String("repository_path", "", "the path of the repository to work in")
	cmd.MarkFlagRequired("repository_path")

	recurseSubmodules := cmd.Flags().Bool("recurse", false, "if true, will recursively update all submodules")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		return updateSubmodules(ctx, *repositoryPath, *mirrorBaseDir, *recurseSubmodules, 0)
	})

	return cmd
}

func updateSubmodules(ctx context.Context, repoPath string, mirrorBaseDir string, recurseSubmodules bool, recursionDepth int) error {
	if recursionDepth > 100 {
		return fmt.Errorf("Reached max nesting level: %d", recursionDepth)
	}
	ctx = withIndent(ctx, recursionDepth)

	if err := checkRepoDir(repoPath); err != nil {
		return err
	}

	submodules, err := getSubmodules(ctx, repoPath)
	if err != nil {
		return err
	}

	// TODO: we could parallelize this, but watch out if the same submodule remote URL is
	// referenced multiple times.
	for _, submod := range submodules {
		indentedFprintf(ctx, console.Info(ctx), "Processing submodule %s -> %s\n", submod.relativePath, submod.remoteUrl)
		mirrorDir, err := ensureMirror(ctx, mirrorBaseDir, submod)
		if err != nil {
			return err
		}

		// Actually get the submodule, using the mirror as reference
		cmd := inRepoGit(repoPath, "submodule", "update", "--init", "--reference", mirrorDir, submod.relativePath)
		err = runAndPrintIfFails(ctx, cmd)
		if err != nil {
			return fmt.Errorf("could not update submodule '%s': %v", submod.relativePath, err)
		}

		if recurseSubmodules {
			recursePath := filepath.Join(repoPath, submod.relativePath)
			indentedFprintf(ctx, console.Debug(ctx), "Recursing into %s (%s)\n", submod.relativePath, recursePath)
			if err := updateSubmodules(ctx, recursePath, mirrorBaseDir, recurseSubmodules, recursionDepth+1); err != nil {
				return err
			}
			indentedFprintf(ctx, console.Debug(ctx), "Left %s\n", submod.relativePath)
		}
	}

	return nil
}

func ensureMirror(ctx context.Context, mirrorBaseDir string, submod submodule) (string, error) {
	mirrorDir := getMirrorDir(mirrorBaseDir, submod)

	if err := os.MkdirAll(mirrorDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("can not create '%s' for '%s': %v", mirrorDir, submod.remoteUrl, err)
	}

	if isMirrorRepo(mirrorDir) {
		// Make sure the mirror is up to date
		cmd := inRepoGit(mirrorDir, "fetch", "--no-recurse-submodules", "origin")
		err := runAndPrintIfFails(ctx, cmd)
		if err != nil {
			return "", fmt.Errorf("could not git fetch '%s' in '%s': %v", submod.remoteUrl, mirrorDir, err)
		}
	} else {
		// Create new mirror
		cmd := exec.Command("git", "clone", "--mirror", "--", submod.remoteUrl, mirrorDir)
		err := runAndPrintIfFails(ctx, cmd)
		if err != nil {
			return "", fmt.Errorf("could not git clone '%s' to '%s': %v", submod.remoteUrl, mirrorDir, err)
		}
	}

	return mirrorDir, nil
}

func isMirrorRepo(mirrorDir string) bool {
	// Mirror repos are bare repositories, so check for that.
	// This not failing implies that mirrorDir is a git repository in the first place.
	err := inRepoGit(mirrorDir, "rev-parse", "--is-bare-repository").Run()
	return err == nil
}

func getMirrorDir(mirrorBaseDir string, mod submodule) string {
	charsToReplace := regexp.MustCompile("[^[:alnum:]-_\\.]")
	key := "submod-" + charsToReplace.ReplaceAllString(mod.remoteUrl, "_")
	return filepath.Join(mirrorBaseDir, "v2", key)
}

func checkRepoDir(repoPath string) error {
	stat, err := os.Stat(repoPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path '%s' does not exist", repoPath)
		} else {
			return fmt.Errorf("i/o error when checking '%s'", repoPath)
		}
	}
	if !stat.IsDir() {
		return fmt.Errorf("path '%s' is not a directory", repoPath)
	}
	return nil
}

// Executes "git" as if it ran in "repoPath" in a different way :-)
// Becuase git submodule doesn't seem to support --work-tree.
func inRepoGit(repoPath string, args ...string) *exec.Cmd {
	allArgs := append(
		[]string{"-C", repoPath},
		args...)

	return exec.Command("git", allArgs...)
}

func getSubmodules(ctx context.Context, repoPath string) ([]submodule, error) {
	cmd := inRepoGit(repoPath, "config", "--file", ".gitmodules", "--get-regexp", "submodule\\.")
	indentedFprintf(ctx, console.Debug(ctx), "exec: %s\n", strings.Join(cmd.Args, " "))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return []submodule{}, err
	}
	scanner := bufio.NewScanner(stdout)
	err = cmd.Start()
	if err != nil {
		return []submodule{}, err
	}

	submoduleMap := map[string]*submodule{}
	// For each submodule, this should produce at least
	// submodule.<submodule-config-key>.path <path-in-repo>
	// submodule.<submodule-config-key>.url <remote-url>
	for scanner.Scan() {
		line := scanner.Text()
		ok, key, value := parseGitConfigKeyValue(line)
		if !ok {
			return []submodule{}, fmt.Errorf("could not parse git config output line '%s'", line)
		}
		ok, submoduleConfigKey, submoduleAttrName := parseSubmoduleConfigKey(key)
		if !ok {
			return []submodule{}, fmt.Errorf("could not parse git config submodule line '%s'", line)
		}

		entry, ok := submoduleMap[submoduleConfigKey]
		if !ok {
			entry = &submodule{}
			entry.configKey = submoduleConfigKey
			submoduleMap[submoduleConfigKey] = entry
		}

		switch submoduleAttrName {
		case "path":
			entry.relativePath = value
		case "url":
			entry.remoteUrl = value
		}
	}
	if scanner.Err() != nil {
		cmd.Process.Kill()
		cmd.Wait()
		return []submodule{}, scanner.Err()
	}
	if err := cmd.Wait(); err != nil {
		return []submodule{}, scanner.Err()
	}

	res := make([]submodule, 0, len(submoduleMap))
	for _, submodule := range submoduleMap {
		res = append(res, *submodule)
	}
	return res, nil
}

// Parses a "git config" output line such as
// key value
// (e.g. "submodule.<submodule-config-key>.path <path-in-repo>")
// returning (ok, key, value)
func parseGitConfigKeyValue(line string) (bool, string, string) {
	separator := strings.IndexByte(line, ' ')
	if separator < 0 {
		return false, "", ""
	}
	key := line[0:separator]
	value := line[separator+1 : len(line)]

	return true, key, value
}

// Parses a "git config" submodule key such as "submodule.<submodule-config-key>.subkey"
// returning (ok, submodule-config-key, subkey)
func parseSubmoduleConfigKey(key string) (bool, string, string) {
	split := strings.Split(key, ".")
	if len(split) != 3 {
		return false, "", ""
	}
	if split[0] != "submodule" {
		return false, "", ""
	}
	return true, split[1], split[2]
}

func runAndPrintIfFails(ctx context.Context, cmd *exec.Cmd) error {
	indentedFprintf(ctx, console.Debug(ctx), "exec: %s\n", strings.Join(cmd.Args, " "))
	output, err := cmd.CombinedOutput()
	if err != nil {
		indentedFprintf(ctx, console.Errors(ctx), "failed: %s\n", strings.Join(cmd.Args, " "))
		fmt.Fprintln(console.Errors(ctx), string(output))
		return err
	}
	return nil
}

func indentedFprintf(ctx context.Context, w io.Writer, format string, a ...any) (n int, err error) {
	indentPrefix := strings.Repeat("  ", indentFromContext(ctx))
	return fmt.Fprintf(w, indentPrefix+format, a...)
}

func withIndent(ctx context.Context, indent int) context.Context {
	return context.WithValue(ctx, _indentKey, indent)
}

func indentFromContext(ctx context.Context) int {
	v := ctx.Value(_indentKey)
	if v == nil {
		return 0
	}
	return v.(int)
}
