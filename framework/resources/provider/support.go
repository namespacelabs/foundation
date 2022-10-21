// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package provider

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"namespacelabs.dev/foundation/framework/resources"
)

func MustPrepare(intent any) (context.Context, *resources.Parsed) {
	intentFlag := flag.String("intent", "", "The serialized JSON intent.")
	resourcesFlag := flag.String("resources", "", "The serialized JSON resources.")

	flag.Parse()

	resources, err := prepare(*intentFlag, *resourcesFlag, intent)
	if err != nil {
		log.Fatal(err.Error())
	}

	return context.Background(), resources
}

func EmitResult(instance any) {
	serialized, err := json.Marshal(instance)
	if err != nil {
		log.Fatalf("failed to marshal instance: %v", err)
	}

	fmt.Printf("namespace.provision.result: %s\n", serialized)
}

func prepare(intentFlag, resourcesFlag string, intent any) (*resources.Parsed, error) {
	if intentFlag == "" {
		return nil, fmt.Errorf("--intent is required")
	}

	if err := json.Unmarshal([]byte(intentFlag), intent); err != nil {
		return nil, fmt.Errorf("failed to decode intent: %w", err)
	}

	if resourcesFlag == "" {
		return nil, fmt.Errorf("--resources is required")
	}

	return resources.ParseResourceData([]byte(resourcesFlag))
}
