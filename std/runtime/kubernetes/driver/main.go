// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import "time"

const maxDuration time.Duration = 1<<63 - 1

func main() {
	// The driver never completes.
	// It relies on getting cancelled by `kubectl run --rm`
	// https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands#run
	for {
		time.Sleep(maxDuration)
	}
}
