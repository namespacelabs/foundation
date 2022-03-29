// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package colors

import "github.com/morikuni/aec"

func Faded(str string) string {
	return aec.LightBlackF.Apply(str)
}

func Bold(str string) string {
	return aec.Bold.Apply(str)
}

func Green(str string) string {
	return aec.GreenF.Apply(str)
}
