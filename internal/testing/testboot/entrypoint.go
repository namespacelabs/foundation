// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package testboot

import (
	"flag"
	"io/ioutil"
	"log"
	"time"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/schema"
)

var (
	testTimeout = flag.Duration("test_timeout", 5*time.Minute, "The maximum duration of the test.")
	debug       = flag.Bool("debug", false, "Output additional test runtime information.")
)

type TestData struct {
	Request *TestRequest
}

func (t TestData) MustEndpoint(owner, name string) *schema.Endpoint {
	for _, endpoint := range t.Request.Endpoint {
		if endpoint.EndpointOwner == owner && endpoint.ServiceName == name {
			return endpoint
		}
	}

	log.Fatalf("Expected endpoint to be present in the stack: endpoint_owner=%q service_name=%q", owner, name)
	return nil
}

func (t TestData) InternalOf(serverOwner string) []*schema.InternalEndpoint {
	var filtered []*schema.InternalEndpoint
	for _, ie := range t.Request.InternalEndpoint {
		if ie.ServerOwner == serverOwner {
			filtered = append(filtered, ie)
		}
	}
	return filtered
}

func BootstrapTest() TestData {
	flag.Parse()

	go func() {
		time.Sleep(*testTimeout)
		log.Fatal("test timed out after", *testTimeout)
	}()

	reqBytes, err := ioutil.ReadFile("/" + TestRequestPath)
	if err != nil {
		log.Fatal(err)
	}

	req := &TestRequest{}
	if err := proto.Unmarshal(reqBytes, req); err != nil {
		log.Fatal(err)
	}

	if *debug {
		log.Println(prototext.Format(req))
		log.Println("initialization complete")
	}

	return TestData{Request: req}
}
