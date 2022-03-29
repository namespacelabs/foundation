// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

//go:build linux
// +build linux

package disk

import "syscall"

var knownFS = map[uint64]string{
	0xef53:     "ext4",
	0x2fc12fc1: "zfs",
}

func FSType(path string) (string, error) {
	s := syscall.Statfs_t{}
	err := syscall.Statfs(path, &s)
	if err != nil {
		return "", err
	}
	if known, ok := knownFS[uint64(s.Type)]; ok {
		return known, nil
	}
	return "unknown", nil
}
