// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Forked from github.com/docker/buildx/localstate/localstate.go (v0.32.1).
// Only the builder-removal path used by store.Txn.Remove is kept; the
// save/read helpers and unexported state types are reduced accordingly.

package buildxstore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

const (
	refsDir  = "refs"
	groupDir = "__group__"
)

type localState struct {
	cfg *Config
}

type localStateRef struct {
	GroupRef string `json:",omitempty"`
}

type localStateGroup struct {
	Refs []string
}

func newLocalState(cfg *Config) (*localState, error) {
	if cfg.Dir() == "" {
		return nil, errors.Errorf("config dir empty")
	}
	if err := cfg.MkdirAll(refsDir, 0700); err != nil {
		return nil, err
	}
	return &localState{cfg: cfg}, nil
}

func (ls *localState) RemoveBuilder(builderName string) error {
	if builderName == "" {
		return errors.Errorf("builder name empty")
	}

	dir := filepath.Join(ls.cfg.Dir(), refsDir, builderName)
	if _, err := os.Lstat(dir); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	fis, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, fi := range fis {
		if err := ls.removeBuilderNode(builderName, fi.Name()); err != nil {
			return err
		}
	}

	return os.RemoveAll(dir)
}

func (ls *localState) removeBuilderNode(builderName string, nodeName string) error {
	if builderName == "" {
		return errors.Errorf("builder name empty")
	}
	if nodeName == "" {
		return errors.Errorf("node name empty")
	}

	dir := filepath.Join(ls.cfg.Dir(), refsDir, builderName, nodeName)
	if _, err := os.Lstat(dir); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	fis, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var murefs sync.Mutex
	grefs := make(map[string][]string)
	srefs := make(map[string][]string)
	eg, _ := errgroup.WithContext(context.TODO())
	for _, fi := range fis {
		func(fi os.DirEntry) {
			eg.Go(func() error {
				st, err := ls.readRef(builderName, nodeName, fi.Name())
				if err != nil {
					return err
				}
				if st.GroupRef == "" {
					return nil
				}
				murefs.Lock()
				defer murefs.Unlock()
				if _, ok := grefs[st.GroupRef]; !ok {
					if grp, err := ls.readGroup(st.GroupRef); err == nil {
						grefs[st.GroupRef] = grp.Refs
					}
				}
				srefs[st.GroupRef] = append(srefs[st.GroupRef], fmt.Sprintf("%s/%s/%s", builderName, nodeName, fi.Name()))
				return nil
			})
		}(fi)
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	for gid, refs := range grefs {
		if s, ok := srefs[gid]; ok {
			if len(s) != len(refs) {
				continue
			}
			if err := ls.removeGroup(gid); err != nil {
				return err
			}
		}
	}

	return os.RemoveAll(dir)
}

func (ls *localState) groupDir() string {
	return filepath.Join(ls.cfg.Dir(), refsDir, groupDir)
}

func (ls *localState) readRef(builderName, nodeName, id string) (*localStateRef, error) {
	if builderName == "" || nodeName == "" || id == "" {
		return nil, errors.Errorf("invalid ref")
	}
	dt, err := os.ReadFile(filepath.Join(ls.cfg.Dir(), refsDir, builderName, nodeName, id))
	if err != nil {
		return nil, err
	}
	var st localStateRef
	if err := json.Unmarshal(dt, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func (ls *localState) readGroup(id string) (*localStateGroup, error) {
	dt, err := os.ReadFile(filepath.Join(ls.groupDir(), id))
	if err != nil {
		return nil, err
	}
	var stg localStateGroup
	if err := json.Unmarshal(dt, &stg); err != nil {
		return nil, err
	}
	return &stg, nil
}

func (ls *localState) removeGroup(id string) error {
	if id == "" {
		return errors.Errorf("group ref empty")
	}
	f := filepath.Join(ls.groupDir(), id)
	if _, err := os.Lstat(f); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return os.Remove(f)
}
