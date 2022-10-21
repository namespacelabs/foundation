// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"namespacelabs.dev/foundation/internal/testdata/integrations/resources/classes"
	pb "namespacelabs.dev/foundation/internal/testdata/integrations/resources/classes/protos"
)

var intent = flag.String("intent", "", "The serialized JSON intent.")

func main() {
	_ = flag.String("resources", "", "The serialized JSON resources.")
	flag.Parse()

	if *intent == "" {
		log.Fatal("--intent is missing")
	}

	i := &classes.DatabaseIntent{}
	if err := json.Unmarshal([]byte(*intent), i); err != nil {
		log.Fatal(err)
	}

	out := &pb.DatabaseInstance{Url: "http://test-" + i.Name}

	serialized, err := json.Marshal(out)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("namespace.provision.result: %s\n", serialized)
}
