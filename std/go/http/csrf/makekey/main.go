// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
)

func main() {
	priv := make([]byte, 32)
	if n, _ := rand.Reader.Read(priv); n != 32 {
		log.Fatal("need 32 bytes")
	}
	fmt.Println(hex.EncodeToString(priv))
}