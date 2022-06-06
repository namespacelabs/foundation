// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package protos

import "google.golang.org/protobuf/proto"

func Clone[V proto.Message](msg V) V {
	return proto.Clone(msg).(V)
}
