// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package core

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

func TestEnforceBefore(t *testing.T) {
	initializers := []*Initializer{
		{Package: pkg("a")},
		{Package: pkg("b"), Before: []string{"a"}},
		{Package: pkg("c"), After: []string{"e"}},
		{Package: pkg("d"), Before: []string{"b"}},
		{Package: pkg("e")},
	}

	if _, err := enforceOrder(initializers); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterDebugEndpointsDoesNotExposePprof(t *testing.T) {
	router := mux.NewRouter()
	RegisterDebugEndpoints(router)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want %d", response.Code, http.StatusNotFound)
	}
}

func pkg(name string) *Package { return &Package{PackageName: name} }
