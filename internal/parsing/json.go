// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package parsing

import (
	"bytes"
	"encoding/json"
)

// JSON decoder that maps numbers to json.Number.
// This allows us to recover integer types with native error checking as JSON uses float64 for all numbers by default.
func NewJsonNumberDecoder(s string) *json.Decoder {
	dec := json.NewDecoder(bytes.NewReader([]byte(s)))
	dec.UseNumber()
	return dec
}
