// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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
