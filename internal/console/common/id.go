// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package common

import "github.com/cespare/xxhash/v2"

type IdAndHash struct {
	ID     string
	Digest uint64
}

func IdAndHashFrom(id string) IdAndHash {
	return IdAndHash{ID: id, Digest: xxhash.Sum64String(id)}
}
