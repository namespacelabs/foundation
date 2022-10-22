// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package k3d

import (
	"encoding/json"
	"testing"

	"github.com/docker/docker/api/types"
)

func TestDockerVersionParsing(t *testing.T) {
	for _, test := range []struct {
		ver types.Version
		ok  bool
	}{
		{ver: types.Version{Version: "20.10.13"}, ok: false},
		{ver: types.Version{Version: "20.10.13", Components: []types.ComponentVersion{{Name: "runc", Version: "1.0.3"}}}, ok: true},
		{ver: types.Version{Version: "20.10.5+dfsg1", Components: []types.ComponentVersion{{Name: "runc", Version: "1.0.0~rc93+ds1"}}}, ok: true},
	} {
		dockerOK, runcOK, _ := validateVersions(test.ver)
		if (dockerOK && runcOK) != test.ok {
			serialized, _ := json.Marshal(test.ver)
			t.Errorf("failed to parse, expected ok=%v, got=(%v && %v), ver was %s", test.ok, dockerOK, runcOK, serialized)
		}
	}
}
