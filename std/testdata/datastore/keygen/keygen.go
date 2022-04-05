// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"os"

	"namespacelabs.dev/go-ids"
)

func main() {
	os.Stdout.Write([]byte(ids.NewRandomBase32ID(128)))
}
