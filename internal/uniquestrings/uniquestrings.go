// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package uniquestrings

// Not thread safe.
type List struct {
	index   map[string]bool
	ordered []string
}

func (dl *List) Len() int { return len(dl.ordered) }

func (dl *List) Strings() []string { return dl.ordered }

// Adds the specified string to the set. Returns true if the string was not present in the set.
func (dl *List) Add(v string) bool {
	if dl.index != nil {
		if _, ok := dl.index[v]; ok {
			return false
		}
	} else {
		dl.index = map[string]bool{}
	}

	dl.index[v] = true
	dl.ordered = append(dl.ordered, v)
	return true
}

func (dl *List) Has(v string) bool {
	if dl.index != nil {
		return dl.index[v]
	}
	return false
}
