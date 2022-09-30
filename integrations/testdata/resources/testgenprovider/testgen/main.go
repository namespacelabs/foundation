// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"encoding/json"
	"fmt"
	"log"

	pb "namespacelabs.dev/foundation/integrations/testdata/resources/classes/protos"
)

func main() {
	out := &pb.DatabaseInstance{Url: "http://test"}

	serialized, err := json.Marshal(out)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("namespace.provision.result: %s\n", serialized)
}
