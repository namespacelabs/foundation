// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package iam

import (
	"bytes"
	"fmt"
)

type PolicyDocument struct {
	Version   string
	Statement []StatementEntry
}

type StatementEntry struct {
	Effect    string
	Principal *Principal `json:"Principal,omitempty"`
	Action    []string   `json:"Action,omitempty"`
	Resource  []string   `json:"Resource,omitempty"`
	Condition *Condition `json:"Condition,omitempty"`
}

type Principal struct {
	Federated string `json:"Federated,omitempty"`
	AWS       string `json:"AWS,omitempty"`
}

type Condition struct {
	StringEquals []Condition_KeyValue
}

type Condition_KeyValue struct {
	Key   string
	Value string
}

func (c Condition) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "{%q:{", "StringEquals")
	for k, kv := range c.StringEquals {
		fmt.Fprintf(&b, "%q:%q", kv.Key, kv.Value)
		if k < len(c.StringEquals)-1 {
			fmt.Fprintf(&b, ",")
		}
	}
	fmt.Fprintf(&b, "}}")
	return b.Bytes(), nil
}
