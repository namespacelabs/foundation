// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

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

func (c CodegenError) Error() string {
	return c.Err.Error()
}

func (c CodegenError) fingerprint() string {
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

func (c *ErrorCollector) IsEmpty() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.errs) == 0
}

func (c *ErrorCollector) Build() *CodegenMultiError {
	c.mu.Lock()
	defer c.mu.Unlock()

	return NewCodegenMultiError(c.errs)
}

type packages map[string]struct{}

// CodegenMultiError accumulates multiple CodegenError(s).
type CodegenMultiError struct {
	// accumulated CodenErrors.
	errs []CodegenError

	// aggregates CodegenErrors by their root cause.
	commonerrs map[error]map[string]packages

	// contains errors not grouped in [commonerrs]
	uniqgenerrs []CodegenError
}

// NewCodegenMultiError walks through accumulated CodegenErrors aggregating
// those errors whose chains share a common root.
func NewCodegenMultiError(errs []CodegenError) *CodegenMultiError {
	// tracks duplicate CodegenErrors by fingerprint.
	commonerrs := make(map[error]map[string]packages)
	duplicateerrs := make(map[string]struct{})

	for i := 0; i < len(errs); i++ {
		for j := 0; j < len(errs); j++ {
			c1 := errs[i]
			c2 := errs[j]
			err1 := c1.Err
			err2 := c2.Err
			if rootErr := commonRootError(err1, err2); rootErr != nil {
				addCommonError(commonerrs, rootErr, c1)
				addCommonError(commonerrs, rootErr, c1)

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
	// Find all unique CodegenErrors that don't have a common root.
	var uniqgenerrs []CodegenError
	for _, generr := range errs {
		_, duplicate := duplicateerrs[generr.fingerprint()]
		if !duplicate {
			uniqgenerrs = append(uniqgenerrs, generr)
		}
	}

	err := &CodegenMultiError{errs: errs, commonerrs: commonerrs, uniqgenerrs: uniqgenerrs}
	return err
}

func (c *CodegenMultiError) Error() string {
	buf := &bytes.Buffer{}
	for _, err := range c.errs {
		fmt.Fprintf(buf, "%s\n", err.Error())
	}
	return buf.String()
}

func addCommonError(commonerrs map[error]map[string]packages, root error, err CodegenError) {
	_, has := commonerrs[root]
	if !has {
		commonerrs[root] = make(map[string]packages)
	}
	whatpkgs := commonerrs[root]

	_, has = whatpkgs[err.What]
	if !has {
		whatpkgs[err.What] = make(packages)
	}
	pkgs := whatpkgs[err.What]
	pkgs[err.PackageName] = struct{}{}
}

// commonRootError unwraps the chains of err1 and err2
// till a common root error is encountered. Returns nil if
// there is no common root error.
func commonRootError(err1 error, err2 error) error {
	if err1 == err2 {
		return nil
	}
	x := err1
	for {
		y := err2
		for {
			if x == y {
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
