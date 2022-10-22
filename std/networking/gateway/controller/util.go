// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"net"
	"strconv"
)

type AddressPort struct {
	addr string
	port uint32
}

func ParseAddressPort(serverAddress string) (*AddressPort, error) {
	addr, portStr, err := net.SplitHostPort(serverAddress)
	if err != nil {
		return nil, err
	}

	port, err := strconv.ParseInt(portStr, 10, 32)
	if err != nil {
		return nil, err
	}

	return &AddressPort{addr, uint32(port)}, nil
}
