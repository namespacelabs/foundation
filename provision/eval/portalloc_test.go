// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eval

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"namespacelabs.dev/foundation/schema"
)

func TestPortAllocator(t *testing.T) {
	var allocs PortAllocations

	alloc := MakePortAllocator(&schema.Server{PackageName: "foobar"}, PortRange{Base: 40000, Max: 41000}, &allocs)

	for i := 0; i < 5; i++ {
		if _, err := alloc(context.Background(), nil, &schema.Need{Type: &schema.Need_Port_{Port: &schema.Need_Port{Name: fmt.Sprintf("port%d", i)}}}); err != nil {
			t.Error(err)
		}
	}

	if d := cmp.Diff([]*schema.Endpoint_Port{
		{Name: "port0", ContainerPort: 40382},
		{Name: "port1", ContainerPort: 40976},
		{Name: "port2", ContainerPort: 40647},
		{Name: "port3", ContainerPort: 40814},
		{Name: "port4", ContainerPort: 40645},
	}, allocs.Ports, protocmp.Transform()); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}
