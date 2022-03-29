// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package production

import (
	"fmt"

	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/workspace/pins"
)

func NonRootRunAs(server string) *runtime.RunAs {
	srv := pins.Server(server)
	if srv == nil || srv.NonRootUserID == nil {
		return nil
	}

	return NonRootRunAsWithID(*srv.NonRootUserID)
}

func NonRootRunAsWithID(id int) *runtime.RunAs {
	return &runtime.RunAs{
		UserID: fmt.Sprintf("%d", id),
	}
}