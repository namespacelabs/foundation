// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"fmt"
	"log"
	"net/http"

	"namespacelabs.dev/foundation/framework/runtime"
)

func main() {
	config, err := runtime.LoadRuntimeConfig()
	if err != nil {
		log.Fatal(err)
	}

	port := config.Current.Port[0].Port

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("Hello, world! From Go"))
	})

	log.Printf("Listening on port: %d", port)

	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
