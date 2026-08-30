// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Forked from github.com/docker/buildx/store/util.go (v0.32.1).

package buildxstore

import (
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

var namePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9\.\-_]*$`)

type errInvalidName struct {
	error
}

func (e *errInvalidName) Error() string {
	return e.error.Error()
}

func (e *errInvalidName) Unwrap() error {
	return e.error
}

func IsErrInvalidName(err error) bool {
	_, ok := err.(*errInvalidName)
	return ok
}

func ValidateName(s string) (string, error) {
	if !namePattern.MatchString(s) {
		return "", &errInvalidName{
			errors.Errorf("invalid name %s, name needs to start with a letter and may not contain symbols, except ._-", s),
		}
	}
	return strings.ToLower(s), nil
}
