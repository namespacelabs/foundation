// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package timefmt

import (
	"fmt"
	"time"
)

func Format(dur time.Duration) string {
	if micros := dur.Microseconds(); micros < 1000 {
		return fmt.Sprintf("%dus", micros)
	}

	if mil := dur.Milliseconds(); mil < 1000 {
		return fmt.Sprintf("%dms", mil)
	}

	return Seconds(dur)
}

func Seconds(dur time.Duration) string {
	return fmt.Sprintf("%.1fs", dur.Seconds())
}
