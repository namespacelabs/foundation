// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"log"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var ()

func main() {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("failed to create incluster config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("failed to create incluster clientset: %v", err)
	}

	ctx := context.Background()

	go controlEphemeral(ctx, clientset)
	go controlDev(ctx, clientset)
}
