// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package common

import (
	"encoding/json"
	"fmt"
)

func Serialize(msg interface{}) (interface{}, error) {
	if s, ok := msg.(SerializableArgument); ok {
		return s.SerializeAsJSON()
	}
	if s, ok := msg.(fmt.Stringer); ok {
		return s.String(), nil
	}
	return msg, nil
}

func SerializeToBytes(msg interface{}) ([]byte, error) {
	return json.Marshal(msg)
}

type SerializableArgument interface {
	SerializeAsJSON() (interface{}, error)
}
