// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeparser

import (
	"embed"
	"testing"
)

var (
	//go:embed testdata/*.yaml
	lib embed.FS
)

func TestFromReader(t *testing.T) {
	one, err := lib.Open("testdata/1.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer one.Close()

	list, err := MultipleFromReader("nginx", one)
	if err != nil {
		t.Error(err)
	}

	if len(list) != 19 {
		t.Errorf("expected %d items, got %d", 19, len(list))
	}
}
