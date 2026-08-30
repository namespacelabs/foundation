// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

func main() {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Listening on %v", ln.Addr())

	s := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Failed")
	})}

	if err := s.Serve(ln); err != nil {
		log.Fatal(err)
	}
}
