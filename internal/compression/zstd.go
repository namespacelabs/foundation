// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package compression

import (
	"bytes"
	"io"

	"github.com/klauspost/compress/zstd"
)

func DecompressZstd(payload []byte) ([]byte, error) {
	r, err := zstd.NewReader(bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(r)
}
