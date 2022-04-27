// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package naming

import (
	"crypto/sha256"
	"encoding/base32"
	"fmt"
)

const (
	lowerCaseEncodeBase32 = "0123456789abcdefghijklmnopqrstuv"
)

var (
	base32encoding = base32.NewEncoding(lowerCaseEncodeBase32).WithPadding(base32.NoPadding)
)

func StableID(str string, n int) string {
	h := sha256.New()
	fmt.Fprint(h, str)
	digest := h.Sum(nil)

	return base32encoding.EncodeToString(digest)[:n]
}
