// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"log"
	"os"
	"os/exec"
	"strconv"

	"namespacelabs.dev/foundation/std/development/filesync"
)

func main() {
	// Args structure is [root] [port] [server provided flags]
	log.SetFlags(log.Lmicroseconds | log.LstdFlags)

	if len(os.Args) < 4 {
		log.Fatal("must be past at least root, port and server")
	}

	root := os.Args[1]
	portStr := os.Args[2]
	command := os.Args[3]
	args := os.Args[4:]

	port, err := strconv.ParseInt(portStr, 10, 32)
	if err != nil {
		log.Fatalf("failed to parse port: %v", err)
	}

	go filesync.StartFileSyncServer(&filesync.FileSyncConfiguration{
		RootDir: root,
		Port:    int32(port),
	})

	cmd := exec.Command(command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Directory and env are inherited.

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}

		log.Fatal(err)
	}
}
