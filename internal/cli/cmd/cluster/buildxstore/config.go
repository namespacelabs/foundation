// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Package buildxstore is a minimal, faithful fork of the builder store from
// github.com/docker/buildx (store, store/storeutil, util/confutil,
// util/dockerutil, util/platformutil and localstate), at version v0.32.1.
//
// nsc only needs to register and unregister Namespace remote builders into the
// user's `docker buildx` configuration. The upstream module pulls in a very
// large dependency tree, so we vendor just the pieces we use here.
//
// IMPORTANT: the on-disk format (under ${DOCKER_CONFIG}/buildx, i.e.
// instances/, defaults/, activity/ and refs/) MUST stay byte-compatible with
// docker buildx so that builders configured by nsc remain usable by the
// `docker buildx` CLI and vice-versa. When updating this fork, diff against
// https://github.com/docker/buildx/tree/v0.32.1.
package buildxstore

import (
	"os"
	"path/filepath"

	"github.com/docker/cli/cli/command"
	"github.com/moby/sys/atomicwriter"
	fs "github.com/tonistiigi/fsutil/copy"
)

// Config resolves and writes to the buildx configuration directory.
// Forked from github.com/docker/buildx/util/confutil.
type Config struct {
	dir     string
	chowner *chowner
}

type chowner struct {
	uid int
	gid int
}

// NewConfig resolves the buildx config dir; if `$BUILDX_CONFIG` is set it is
// used, otherwise the parent directory of the Docker config file is used (i.e.
// `${DOCKER_CONFIG}/buildx`).
func NewConfig(dockerCli command.Cli) *Config {
	configDir := os.Getenv("BUILDX_CONFIG")
	if configDir == "" {
		configDir = filepath.Join(filepath.Dir(dockerCli.ConfigFile().Filename), "buildx")
	}

	return &Config{
		dir:     configDir,
		chowner: sudoer(configDir),
	}
}

// Dir returns the configuration store path.
func (c *Config) Dir() string {
	return c.dir
}

// MkdirAll creates a directory and all necessary parents within the config dir.
func (c *Config) MkdirAll(dir string, perm os.FileMode) error {
	var chown fs.Chowner
	if c.chowner != nil {
		chown = func(user *fs.User) (*fs.User, error) {
			return &fs.User{UID: c.chowner.uid, GID: c.chowner.gid}, nil
		}
	}
	d := filepath.Join(c.dir, dir)
	st, err := os.Stat(d)
	if err != nil {
		if os.IsNotExist(err) {
			_, err := fs.MkdirAll(d, perm, chown, nil)
			return err
		}
		return err
	}
	// if directory already exists, fix the owner if necessary
	if c.chowner == nil {
		return nil
	}
	currentOwner := fileOwner(st)
	if currentOwner != nil && (currentOwner.uid != c.chowner.uid || currentOwner.gid != c.chowner.gid) {
		return os.Chown(d, c.chowner.uid, c.chowner.gid)
	}
	return nil
}

// AtomicWriteFile writes data to a file within the config dir atomically.
func (c *Config) AtomicWriteFile(filename string, data []byte, perm os.FileMode) error {
	f := filepath.Join(c.dir, filename)
	if err := atomicwriter.WriteFile(f, data, perm); err != nil {
		return err
	}
	if c.chowner == nil {
		return nil
	}
	return os.Chown(f, c.chowner.uid, c.chowner.gid)
}
