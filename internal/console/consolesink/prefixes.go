// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package consolesink

import (
	"strings"

	"github.com/morikuni/aec"
)

var (
	prefix0 = " => "
	prefix1 = " " + dim(80, "=") + dim(150, "=> ")
	prefix2 = " " + dim(80, "=") + dim(110, "=") + dim(130, "=> ")
	prefix3 = " " + dim(40, "=") + dim(60, "=") + dim(80, "=") + dim(100, "=> ")
)

func dim(n int, str string) string {
	return aec.Color8BitF(aec.NewRGB8Bit(uint8(n), uint8(n), uint8(n))).Apply(str)
}

func renderPrefix(depth int) string {
	switch depth {
	case 0:
		return prefix0
	case 1:
		return prefix1
	case 2:
		return prefix2
	case 3:
		return prefix3
	default:
		return strings.Repeat(" ", depth-4) + prefix3
	}
}
