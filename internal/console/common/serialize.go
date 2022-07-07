// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package common

import (
	"encoding/json"
)

func SerializeToBytes(msg interface{}) ([]byte, error) {
	return json.Marshal(msg)
}
