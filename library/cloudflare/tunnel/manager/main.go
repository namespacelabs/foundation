// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"flag"
	"log"
	"os"
	"os/exec"

	conf "namespacelabs.dev/foundation/library/cloudflare/tunnel/configuration"
)

var (
	credentials   = flag.String("credentials", "", "Path to credentials file.")
	configuration = flag.String("configuration", "", "Path to the configuration file.")
	cloudflared   = flag.String("cloudflared", "/usr/local/bin/cloudflared", "Path to cloudflared.")
)

func main() {
	flag.Parse()

	if *credentials == "" {
		log.Fatal("--credentials is required")
	}

	tunnelID, err := conf.ReadTunnelID(*credentials)
	if err != nil {
		log.Fatal(err)
	}

	cmd := exec.Command(*cloudflared, "tunnel", "--config", *configuration, "run", tunnelID.String())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}
