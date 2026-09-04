// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"errors"
	"testing"

	"github.com/bazelbuild/remote-apis-sdks/go/pkg/rexec"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestIsNamespaceRegistry(t *testing.T) {
	for _, test := range []struct {
		registry string
		want     bool
	}{
		{registry: "nscr.io", want: true},
		{registry: "tenant.nscr.io", want: true},
		{registry: "notnscr.io", want: false},
		{registry: "registry.example.com", want: false},
	} {
		if got := isNamespaceRegistry(test.registry); got != test.want {
			t.Errorf("isNamespaceRegistry(%q) = %v, want %v", test.registry, got, test.want)
		}
	}
}

func TestRemoteUploadMetadataSurvivesLayerReplacement(t *testing.T) {
	contents := []byte("compressed layer")
	original := static.NewLayer(contents, types.OCILayer)
	executor := &rexec.Client{}
	wrapped, err := WithBazelRemoteUpload(original, executor)
	if err != nil {
		t.Fatal(err)
	}

	if remoteLayer, ok, err := remoteUploadLayer(wrapped); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.Fatal("wrapped layer is not marked for remote upload")
	} else if remoteLayer.executor != executor {
		t.Fatal("wrapped layer lost its remote executor")
	}

	// OCI image mutations can recreate a layer object from its digest. The marker
	// must follow the immutable content rather than depend on Go object identity.
	replacement := static.NewLayer(contents, types.OCILayer)
	remoteLayer, ok, err := remoteUploadLayer(replacement)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("replacement layer lost remote upload metadata")
	}
	if remoteLayer.executor != executor {
		t.Fatal("replacement layer lost its remote executor")
	}
}

func TestRemoteUploadLayerPropagatesDigestErrors(t *testing.T) {
	wantErr := errors.New("digest failed")
	layer := digestErrorLayer{
		Layer: static.NewLayer([]byte("compressed layer"), types.OCILayer),
		err:   wantErr,
	}
	if _, err := WithBazelRemoteUpload(layer, &rexec.Client{}); !errors.Is(err, wantErr) {
		t.Fatalf("WithBazelRemoteUpload() error = %v, want %v", err, wantErr)
	}
	if _, _, err := remoteUploadLayer(layer); !errors.Is(err, wantErr) {
		t.Fatalf("remoteUploadLayer() error = %v, want %v", err, wantErr)
	}
}

type digestErrorLayer struct {
	v1.Layer
	err error
}

func (layer digestErrorLayer) Digest() (v1.Hash, error) {
	return v1.Hash{}, layer.err
}
