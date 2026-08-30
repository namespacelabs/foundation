// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestParseGitConfigKeyValue(t *testing.T) {
	tests := []struct {
		input     string
		wantOk    bool
		wantKey   string
		wantValue string
	}{
		{"submodule.foo.path src/foo", true, "submodule.foo.path", "src/foo"},
		{"submodule.foo.url git@github.com:org/repo.git", true, "submodule.foo.url", "git@github.com:org/repo.git"},
		{"key value with spaces", true, "key", "value with spaces"},
		{"nospace", false, "", ""},
		{"", false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ok, key, value := parseGitConfigKeyValue(tt.input)
			if ok != tt.wantOk || key != tt.wantKey || value != tt.wantValue {
				t.Errorf("parseGitConfigKeyValue(%q) = (%v, %q, %q), want (%v, %q, %q)",
					tt.input, ok, key, value, tt.wantOk, tt.wantKey, tt.wantValue)
			}
		})
	}
}

func TestParseSubmoduleConfigKey(t *testing.T) {
	tests := []struct {
		input         string
		wantOk        bool
		wantConfigKey string
		wantAttrName  string
	}{
		{"submodule.mylib.path", true, "mylib", "path"},
		{"submodule.mylib.url", true, "mylib", "url"},
		{"submodule.some-project-1.9.path", true, "some-project-1.9", "path"},
		{"submodule.some-project-1.9.url", true, "some-project-1.9", "url"},
		{"submodule.lib-1.2.3-rc.1.path", true, "lib-1.2.3-rc.1", "path"},
		{"submodule.a.b.c.path", true, "a.b.c", "path"},
		{"other.foo.path", false, "", ""},
		{"submodule.path", false, "", ""},
		{"submodule..path", false, "", ""},
		{"submodule.foo.", false, "", ""},
		{"nodots", false, "", ""},
		{"", false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ok, configKey, attrName := parseSubmoduleConfigKey(tt.input)
			if ok != tt.wantOk || configKey != tt.wantConfigKey || attrName != tt.wantAttrName {
				t.Errorf("parseSubmoduleConfigKey(%q) = (%v, %q, %q), want (%v, %q, %q)",
					tt.input, ok, configKey, attrName, tt.wantOk, tt.wantConfigKey, tt.wantAttrName)
			}
		})
	}
}

// TestUpdateSubmodulesNoDeadlock verifies that updateSubmodules returns promptly
// when mirror clones fail, rather than deadlocking on unbuffered channel sends
// after the errgroup context is cancelled.
func TestUpdateSubmodulesNoDeadlock(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a main repo with multiple submodules pointing to non-existent URLs.
	// Multiple submodules are needed to trigger the deadlock: one worker hits an
	// error while another is blocked sending on an unbuffered channel.
	repoDir := filepath.Join(tmpDir, "repo")
	mirrorDir := filepath.Join(tmpDir, "mirror")
	os.MkdirAll(mirrorDir, os.ModePerm)

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		// Allow local file:// transport for submodule setup.
		cmd.Env = append(os.Environ(), "GIT_PROTOCOL_FROM_USER=0", "GIT_CONFIG_COUNT=1",
			"GIT_CONFIG_KEY_0=protocol.file.allow", "GIT_CONFIG_VALUE_0=always")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	run(tmpDir, "git", "init", repoDir)
	run(repoDir, "git", "config", "user.email", "test@test.com")
	run(repoDir, "git", "config", "user.name", "Test")

	// Create an initial commit so HEAD exists, and set a remote origin.
	run(repoDir, "git", "commit", "--allow-empty", "-m", "init")
	run(repoDir, "git", "remote", "add", "origin", "https://github.com/nonexistent/repo.git")

	// Create multiple submodule entries in .gitmodules pointing to bad URLs.
	submodNames := []string{"sub-a", "sub-b", "sub-c", "sub-d"}

	// For each submodule, create a real gitlink by adding a local repo as submodule.
	for _, name := range submodNames {
		fakeRemote := filepath.Join(tmpDir, "fake-"+name)
		run(tmpDir, "git", "init", fakeRemote)
		run(fakeRemote, "git", "config", "user.email", "test@test.com")
		run(fakeRemote, "git", "config", "user.name", "Test")
		run(fakeRemote, "git", "commit", "--allow-empty", "-m", "init")
		run(repoDir, "git", "submodule", "add", "--name", name, fakeRemote, name)
	}

	// Overwrite .gitmodules URLs to point to non-existent repos.
	gitmodules := ""
	for _, name := range submodNames {
		gitmodules += "[submodule \"" + name + "\"]\n"
		gitmodules += "\tpath = " + name + "\n"
		gitmodules += "\turl = file:///nonexistent-repo-" + name + "\n"
	}
	os.WriteFile(filepath.Join(repoDir, ".gitmodules"), []byte(gitmodules), 0644)
	run(repoDir, "git", "add", "-A")
	run(repoDir, "git", "commit", "-m", "add submodules")

	maxAttempts := 1
	p := &processor{
		repoPath:        repoDir,
		mirrorBaseDir:   mirrorDir,
		numWorkers:      4,
		repoBufLen:      1000,
		maxRecurseDepth: 20,
		maxAttempts:     &maxAttempts,
	}

	ctx := context.Background()

	done := make(chan error, 1)
	go func() {
		done <- p.updateSubmodules(ctx)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error from updateSubmodules, got nil")
		}
		// Success: returned an error without deadlocking.
	case <-time.After(10 * time.Second):
		t.Fatal("updateSubmodules deadlocked: did not return within 10 seconds")
	}
}
