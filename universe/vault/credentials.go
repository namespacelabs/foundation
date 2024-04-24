// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package vault

import (
	"encoding/json"
	"os"
)

type Credentials struct {
	AppRole  string `json:"app_role"`
	SecretId string `json:"secret_id"`
}

func ParseCredentials(data []byte) (*Credentials, error) {
	c := Credentials{}
	return &c, json.Unmarshal(data, &c)
}

func ParseCredentialsFromEnv(key string) (*Credentials, error) {
	return ParseCredentials([]byte(os.Getenv(key)))
}

func (c Credentials) Encode() ([]byte, error) {
	return json.Marshal(c)
}
