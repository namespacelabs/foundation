// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package jsonreparser

import "encoding/json"

// Reparse takes the unserialized form of an instance, and re-parses it into the target instance.
func Reparse(obj interface{}, target interface{}) error {
	// XXX for now we do a marshal/unmarshal cycle; reconsider this in the future.
	b, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	return json.Unmarshal(b, target)
}
