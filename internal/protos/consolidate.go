// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package protos

import (
	"reflect"

	"google.golang.org/protobuf/proto"
)

func CheckConsolidate[V proto.Message](check V, out *V) bool {
	if reflect.ValueOf(out).Elem().IsNil() {
		*out = check
	} else if !proto.Equal(*out, check) {
		return false
	}
	return true
}
