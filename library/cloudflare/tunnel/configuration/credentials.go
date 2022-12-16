// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package configuration

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
)

type Credentials struct {
	TunnelID string `json:"TunnelID"`
}

func ReadTunnelID(credentialsPath string) (uuid.UUID, error) {
	data, err := os.ReadFile(credentialsPath)
	if err != nil {
		return uuid.Nil, err
	}

	return readTunnelID(data)
}

func ReadTunnelIDFromEnv(env string) (uuid.UUID, error) {
	data := os.Getenv(env)
	if data == "" {
		return uuid.Nil, fmt.Errorf("%q is not set", env)
	}

	return readTunnelID([]byte(data))
}

func readTunnelID(data []byte) (uuid.UUID, error) {
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return uuid.Nil, err
	}

	if creds.TunnelID == "" {
		return uuid.Nil, fmt.Errorf("TunnelID is missing")
	}

	return uuid.Parse(creds.TunnelID)
}
