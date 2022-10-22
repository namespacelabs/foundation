// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnerrors

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
)

// CodegenError associates an error with a code generation phase and package.
type CodegenError struct {
	PackageName string
	What        string
	Err         error
}

func (c *CodegenError) Error() string {
	return c.Err.Error()
}

func (c *CodegenError) Unwrap() error { return c.Err }

func (c *CodegenError) fingerprint() string {
	return fmt.Sprintf("%s:%v:%v", c.What, c.PackageName, c.Err)
}

// A thread-safe container for [CodegenError] that can generate a [CodegenMultiError].
type ErrorCollector struct {
	// guards access to fields below.
	mu sync.Mutex

	// accumulated CodenErrors.
	errs []CodegenError
}

func (c *ErrorCollector) Append(generr CodegenError) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errs = append(c.errs, generr)
}

// Error returns a CodegenMultiError which aggregates all errors that were
// gathered. If no errors were collected, this method returns nil.
func (c *ErrorCollector) Error() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.errs) == 0 {
		return nil
	}

	return newCodegenMultiError(c.errs)
}

type packages map[string]struct{}

// CodegenMultiError accumulates multiple CodegenError(s).
type CodegenMultiError struct {
	// accumulated CodenErrors.
	Errs []CodegenError

	// aggregates CodegenErrors by root error message.
	CommonErrs map[string]map[string]packages

	// contains errors not grouped in [commonerrs].
	UniqGenErrs []CodegenError
}

// newCodegenMultiError walks through accumulated CodegenErrors aggregating
// those errors whose chains share a common root.
func newCodegenMultiError(errs []CodegenError) *CodegenMultiError {
	// tracks duplicate CodegenErrors by fingerprint.
	commonerrs := make(map[string]map[string]packages)
	duplicateerrs := make(map[string]struct{})

	for i := 0; i < len(errs); i++ {
		for j := 0; j < len(errs); j++ {
			c1 := errs[i]
			c2 := errs[j]
			if c1.Err != c2.Err {
				err1 := c1.Err
				err2 := c2.Err
				if rootErr := commonRootError(err1, err2); rootErr != nil {
					addCommonError(commonerrs, rootErr, c1, c2)

					_, has := duplicateerrs[c1.fingerprint()]
					if !has {
						duplicateerrs[c1.fingerprint()] = struct{}{}
					}
					_, has = duplicateerrs[c2.fingerprint()]
					if !has {
						duplicateerrs[c2.fingerprint()] = struct{}{}
					}
				}
			}
		}
	}
	// Find all unique CodegenErrors that don't have a common root.
	var uniqgenerrs []CodegenError
	for _, generr := range errs {
		_, duplicate := duplicateerrs[generr.fingerprint()]
		if !duplicate {
			uniqgenerrs = append(uniqgenerrs, generr)
		}
	}

	err := &CodegenMultiError{Errs: errs, CommonErrs: commonerrs, UniqGenErrs: uniqgenerrs}
	return err
}

func (c *CodegenMultiError) Error() string {
	buf := &bytes.Buffer{}
	for _, err := range c.Errs {
		fmt.Fprintf(buf, "%s\n", err.Error())
	}
	return buf.String()
}

// existingCommonError recursively checks for root or any of it's wrapped errors
// in commonerrors, and returns either nil or the first unwrapped error that is found.
func existingCommonError(commonerrs map[string]map[string]packages, root error) error {
	if root == nil {
		return nil
	}
	if _, has := commonerrs[root.Error()]; has {
		return root
	}
	return existingCommonError(commonerrs, errors.Unwrap(root))
}

func addErrPackage(errpkgs map[string]packages, err CodegenError) {
	if _, has := errpkgs[err.What]; !has {
		errpkgs[err.What] = make(packages)
	}
	pkgs := errpkgs[err.What]
	pkgs[err.PackageName] = struct{}{}
}

// addCommonError associates the given root error with it's child codegen errors.
// If any error in the chain of `root` is already in the commonerrs map, we add the pair of
// codegen errors to this already mapped ancestor.
func addCommonError(commonerrs map[string]map[string]packages, root error, err1 CodegenError, err2 CodegenError) {
	commonerr := existingCommonError(commonerrs, root)
	if commonerr == nil {
		commonerrs[root.Error()] = map[string]packages{}
		commonerr = root
	}
	errpkgs := commonerrs[commonerr.Error()]
	addErrPackage(errpkgs, err1)
	addErrPackage(errpkgs, err2)
}

// commonRootError unwraps the chains of err1 and err2
// till a common root error is encountered. Returns nil if
// there is no common root error.
func commonRootError(err1 error, err2 error) error {
	x := err1
	for {
		y := err2
		for {
			if x.Error() == y.Error() {
				return x
			}
			y = errors.Unwrap(y)
			if y == nil {
				break
			}
		}
		x = errors.Unwrap(x)
		if x == nil {
			break
		}
	}
	return nil
}
