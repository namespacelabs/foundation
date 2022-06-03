// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package provision

import (
	"fmt"
	"strings"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/go-ids"
)

type BuildID struct {
	Version string
	Suffix  string
}

func NewBuildID() BuildID {
	return BuildID{Version: ids.NewRandomBase32ID(8)}
}

func ParseBuildID(v string) (BuildID, error) {
	p := strings.SplitN(v, "-", 2)
	switch len(p) {
	case 1:
		return BuildID{Version: p[0]}, nil
	case 2:
		return BuildID{Version: p[0], Suffix: p[1]}, nil
	default:
		return BuildID{}, fnerrors.InternalError("unsupported format")
	}
}

func (b *BuildID) String() string {
	if b == nil {
		return ""
	}
	if b.Suffix != "" {
		return fmt.Sprintf("%s-%s", b.Version, b.Suffix)
	}
	return b.Version
}

func (b BuildID) WithSuffix(suffix string) BuildID {
	return BuildID{Version: b.Version, Suffix: suffix}
}
