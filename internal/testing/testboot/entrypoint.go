// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package testboot

import (
	"flag"
	"log"
	"os"
	"time"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

type TestData struct {
	Request *TestRequest
}

func BootstrapTest(testTimeout time.Duration, debug bool) TestData {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)

	flag.Parse()

	go func() {
		time.Sleep(testTimeout)
		log.Fatal("test timed out after", testTimeout)
	}()

	reqBytes, err := os.ReadFile("/" + TestRequestPath)
	if err != nil {
		log.Fatal(err)
	}

	req := &TestRequest{}
	if err := proto.Unmarshal(reqBytes, req); err != nil {
		log.Fatal(err)
	}

	if debug {
		log.Println(prototext.Format(req))
		log.Println("initialization complete")
	}

	return TestData{Request: req}
}
