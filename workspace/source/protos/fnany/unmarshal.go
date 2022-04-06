// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnany

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/schema"
)

func CheckUnmarshal(any *anypb.Any, pkg schema.PackageName, msg proto.Message) (bool, error) {
	if any.GetTypeUrl() != TypeURL(pkg, msg) {
		return false, nil
	}

	err := proto.Unmarshal(any.Value, msg)
	return true, err
}
