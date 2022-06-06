// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"flag"
	"log"
	"os"

	"google.golang.org/protobuf/encoding/protojson"
)

var (
	configPath     = flag.String("envoy_config", "/config/envoy.json", "Path to the bootstrap config file.")
	adminAddress   = flag.String("admin_address", ":9091", "The address the Envoy admin endpoint binds to.")
	xdsAddress     = flag.String("xds_address", ":9001", "The address the xDS endpoint binds to.")
	xdsClusterName = flag.String("xds_clustername", "xds_cluster", "The name to use for the xDS management cluster.")
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	flag.Parse()

	xdsAddr, err := NewAddress(*xdsAddress)
	if err != nil {
		log.Fatalf("Failed to parse xDS address: %v", err)
	}

	adminAddr, err := NewAddress(*adminAddress)
	if err != nil {
		log.Fatalf("Failed to parse admin address: %v", err)
	}

	opts := []Option{
		ResourceVersion(ApiVersion_V3),
		ManagementClusterName(*xdsClusterName),
		ManagementAddress(xdsAddr),
		AdminAddress(adminAddr),
	}

	file, err := os.OpenFile(*configPath, os.O_CREATE, 0640)
	if err != nil {
		log.Fatalf("Failed to open config file: %v", err)
	}

	defer func() {
		_ = file.Sync()
		_ = file.Close()
	}()

	bootstrap, err := New(opts...)
	if err != nil {
		log.Fatalf("Failed to create bootstrap: %v", err)
	}

	res, err := (protojson.MarshalOptions{
		UseProtoNames: true,
		Multiline:     false,
	}).Marshal(bootstrap)
	if err != nil {
		log.Fatalf("Failed to marshal bootstrap: %v", err)
	}

	if _, err := file.Write(res); err != nil {
		log.Fatalf("Failed to write bootstrap config: %v", err)
	}
}
