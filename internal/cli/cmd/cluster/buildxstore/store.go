// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Forked from github.com/docker/buildx/store/store.go (v0.32.1). Unused methods
// (List, Current, reset) were dropped; the remaining behaviour and on-disk
// layout are unchanged.

package buildxstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

const (
	instanceDir = "instances"
	defaultsDir = "defaults"
	activityDir = "activity"
)

func New(cfg *Config) (*Store, error) {
	if err := cfg.MkdirAll(instanceDir, 0700); err != nil {
		return nil, err
	}
	if err := cfg.MkdirAll(defaultsDir, 0700); err != nil {
		return nil, err
	}
	if err := cfg.MkdirAll(activityDir, 0700); err != nil {
		return nil, err
	}
	return &Store{cfg: cfg}, nil
}

type Store struct {
	cfg *Config
}

func (s *Store) Txn() (*Txn, func(), error) {
	l := flock.New(filepath.Join(s.cfg.Dir(), ".lock"))
	if err := l.Lock(); err != nil {
		return nil, nil, err
	}
	return &Txn{
			s: s,
		}, func() {
			l.Close()
		}, nil
}

type Txn struct {
	s *Store
}

func (t *Txn) NodeGroupByName(name string) (*NodeGroup, error) {
	name, err := ValidateName(name)
	if err != nil {
		return nil, err
	}
	dt, err := os.ReadFile(filepath.Join(t.s.cfg.Dir(), instanceDir, name))
	if err != nil {
		return nil, err
	}
	var ng NodeGroup
	if err := json.Unmarshal(dt, &ng); err != nil {
		return nil, err
	}
	if ng.LastActivity, err = t.GetLastActivity(&ng); err != nil {
		return nil, err
	}
	return &ng, nil
}

func (t *Txn) Save(ng *NodeGroup) error {
	name, err := ValidateName(ng.Name)
	if err != nil {
		return err
	}
	if err := t.UpdateLastActivity(ng); err != nil {
		return err
	}
	dt, err := json.Marshal(ng)
	if err != nil {
		return err
	}
	return t.s.cfg.AtomicWriteFile(filepath.Join(instanceDir, name), dt, 0600)
}

func (t *Txn) Remove(name string) error {
	name, err := ValidateName(name)
	if err != nil {
		return err
	}
	if err := t.RemoveLastActivity(name); err != nil {
		return err
	}
	ls, err := newLocalState(t.s.cfg)
	if err != nil {
		return err
	}
	if err := ls.RemoveBuilder(name); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(t.s.cfg.Dir(), instanceDir, name))
}

func (t *Txn) SetCurrent(key, name string, global, def bool) error {
	c := current{
		Key:    key,
		Name:   name,
		Global: global,
	}
	dt, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if err := t.s.cfg.AtomicWriteFile("current", dt, 0600); err != nil {
		return err
	}

	h := toHash(key)

	if def {
		if err := t.s.cfg.AtomicWriteFile(filepath.Join(defaultsDir, h), []byte(name), 0600); err != nil {
			return err
		}
	} else {
		os.RemoveAll(filepath.Join(t.s.cfg.Dir(), defaultsDir, h)) // ignore error
	}
	return nil
}

func (t *Txn) UpdateLastActivity(ng *NodeGroup) error {
	return t.s.cfg.AtomicWriteFile(filepath.Join(activityDir, ng.Name), []byte(time.Now().UTC().Format(time.RFC3339)), 0600)
}

func (t *Txn) GetLastActivity(ng *NodeGroup) (la time.Time, _ error) {
	dt, err := os.ReadFile(filepath.Join(t.s.cfg.Dir(), activityDir, ng.Name))
	if err != nil {
		if os.IsNotExist(errors.Cause(err)) {
			return la, nil
		}
		return la, err
	}
	return time.Parse(time.RFC3339, string(dt))
}

func (t *Txn) RemoveLastActivity(name string) error {
	name, err := ValidateName(name)
	if err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(t.s.cfg.Dir(), activityDir, name))
}

type current struct {
	Key    string
	Name   string
	Global bool
}

func toHash(in string) string {
	return digest.FromBytes([]byte(in)).Hex()[:20]
}
