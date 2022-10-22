// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package production

import (
	"fmt"

	"namespacelabs.dev/foundation/internal/dependencies/pins"
	"namespacelabs.dev/foundation/internal/runtime"
)

func NonRootRunAs(server string) *runtime.RunAs {
	srv := pins.Server(server)
	if srv == nil || srv.NonRootUserID == nil {
		return nil
	}

	return NonRootRunAsWithID(*srv.NonRootUserID, srv.FSGroup)
}

func NonRootRunAsWithID(id int, fsGroup *int) *runtime.RunAs {
	runAs := &runtime.RunAs{
		UserID: fmt.Sprintf("%d", id),
	}
	if fsGroup != nil {
		x := fmt.Sprintf("%d", *fsGroup)
		runAs.FSGroup = &x
	}
	return runAs
}
