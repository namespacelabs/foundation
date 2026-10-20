// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tcache_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"namespacelabs.dev/foundation/internal/tcache"
)

func TestComputeRetriesErrors(t *testing.T) {
	cache := tcache.NewCache[string]()
	failure := errors.New("failed")
	_, err := cache.Compute("key", func() (string, error) { return "", failure })
	assert.ErrorIs(t, err, failure)
	value, err := cache.Compute("key", func() (string, error) { return "recovered", nil })
	assert.NoError(t, err)
	assert.Equal(t, "recovered", value)
	value, err = cache.Compute("key", func() (string, error) { return "wrong", nil })
	assert.NoError(t, err)
	assert.Equal(t, "recovered", value)
}

func TestDropIfPreservesReplacement(t *testing.T) {
	cache := tcache.NewCache[string]()
	_, err := cache.Compute("key", func() (string, error) { return "old", nil })
	assert.NoError(t, err)
	dropOld := func(value string) bool { return value == "old" }
	cache.DropIf("key", dropOld)

	started, release := make(chan struct{}), make(chan struct{})
	result := make(chan string, 1)
	go func() {
		value, err := cache.Compute("key", func() (string, error) {
			close(started)
			<-release
			return "new", nil
		})
		assert.NoError(t, err)
		result <- value
	}()
	select {
	case <-started:
	case value := <-result:
		assert.Equal(t, "new", value)
		return
	}
	cache.DropIf("key", func(string) bool { return true })
	close(release)
	assert.Equal(t, "new", <-result)

	cache.DropIf("key", dropOld)
	value, err := cache.Compute("key", func() (string, error) { return "wrong", nil })
	assert.NoError(t, err)
	assert.Equal(t, "new", value)
}
