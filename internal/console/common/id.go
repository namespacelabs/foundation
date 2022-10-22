// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package common

import "github.com/cespare/xxhash/v2"

type IdAndHash struct {
	ID     string
	Digest uint64
}

func IdAndHashFrom(id string) IdAndHash {
	return IdAndHash{ID: id, Digest: xxhash.Sum64String(id)}
}
