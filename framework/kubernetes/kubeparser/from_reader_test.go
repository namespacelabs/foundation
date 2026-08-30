// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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

	list, err := MultipleFromReader("nginx", one, true)
	if err != nil {
		t.Error(err)
	}

	if len(list) != 19 {
		t.Errorf("expected %d items, got %d", 19, len(list))
	}
}
