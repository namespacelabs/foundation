// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"log"
	"os"

	"google.golang.org/protobuf/encoding/protojson"
	"namespacelabs.dev/foundation/std/development/filesync"
)

var (
	// Needs to match the value in the "extension.cue"
	configurationFileName = "./configuration.textpb"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	bytes, err := os.ReadFile(configurationFileName)
	if err != nil {
		log.Fatal(err)
	}

	configuration := &filesync.FileSyncConfiguration{}
	if err := protojson.Unmarshal(bytes, configuration); err != nil {
		log.Fatal(err)
	}

	filesync.StartFileSyncServer(configuration)
}
