// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package compute

type Resolved struct {
	results map[string]ResultWithTimestamp[any]
}

func GetDepWithType[V any](deps Resolved, key string) (ResultWithTimestamp[V], bool) {
	if deps.results == nil {
		return ResultWithTimestamp[V]{}, false
	}

	v, ok := deps.results[key]
	if !ok {
		return ResultWithTimestamp[V]{}, false
	}

	typed, ok := v.Value.(V)
	if !ok {
		return ResultWithTimestamp[V]{}, false
	}

	var r ResultWithTimestamp[V]
	r.Value = typed
	r.Digest = v.Digest
	r.Cached = v.Cached
	r.NonDeterministic = v.NonDeterministic
	r.Timestamp = v.Timestamp
	return r, true
}

func GetDep[V any](deps Resolved, c Computable[V], key string) (ResultWithTimestamp[V], bool) {
	return GetDepWithType[V](deps, key)
}

func MustGetDepValue[V any](deps Resolved, c Computable[V], key string) V {
	v, ok := GetDep(deps, c, key)
	if !ok {
		panic(key + " not present")
	}
	return v.Value
}
