// Copyright 2026 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"encoding/json"
	"log"
	"os"

	"namespacelabs.dev/foundation/framework/runtime"
)

func main() {
	vcs, err := runtime.LoadBuildVCS()
	if err != nil {
		log.Fatal(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(vcs); err != nil {
		log.Fatal(err)
	}
}
