// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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
