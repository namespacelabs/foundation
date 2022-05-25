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

type packages map[string]struct{}

// CodegenMultiError accumulates multiple CodegenError(s).
type CodegenMultiError struct {
	// guards access to fields below.
	mu sync.Mutex

	// accumulated CodenErrors.
	errs []CodegenError

	// aggregates CodegenErrors by their root cause.
	commonerrs map[error]map[string]packages

	// tracks duplicate CodegenErrors by fingerprint.
	duplicateerrs map[string]struct{}
}

func NewCodegenMultiError() *CodegenMultiError {
	return &CodegenMultiError{
		commonerrs:    make(map[error]map[string]packages),
		duplicateerrs: make(map[string]struct{}),
	}
}

func (c *CodegenMultiError) Error() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	buf := &bytes.Buffer{}
	for _, err := range c.errs {
		fmt.Fprintf(buf, "%s\n", err.Error())
	}
	return buf.String()
}

func (c *CodegenMultiError) Append(generr CodegenError) {
	c.mu.Lock()
	c.errs = append(c.errs, generr)
	c.mu.Unlock()
}

func (c *CodegenMultiError) IsEmpty() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.errs) == 0
}

func (c *CodegenMultiError) addCommonError(error error, err1 CodegenError, err2 CodegenError) {
	_, has := c.commonerrs[error]
	if !has {
		c.commonerrs[error] = make(map[string]packages)
	}
	whatpkgs := c.commonerrs[error]

	_, has = whatpkgs[err1.What]
	if !has {
		whatpkgs[err1.What] = make(packages)
	}
	pkgs := whatpkgs[err1.What]
	pkgs[err1.PackageName] = struct{}{}

	_, has = whatpkgs[err2.What]
	if !has {
		whatpkgs[err2.What] = make(packages)
	}
	pkgs = whatpkgs[err2.What]
	pkgs[err2.PackageName] = struct{}{}

	_, has = c.duplicateerrs[err1.fingerprint()]
	if !has {
		c.duplicateerrs[err1.fingerprint()] = struct{}{}
	}
	_, has = c.duplicateerrs[err2.fingerprint()]
	if !has {
		c.duplicateerrs[err2.fingerprint()] = struct{}{}
	}
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

// aggregateErrors walks through accumulated CodegenErrors aggregating
// those errors whose chains share a common root.
func (c *CodegenMultiError) aggregateErrors() {
	for i := 0; i < len(c.errs); i++ {
		for j := 0; j < len(c.errs); j++ {
			c1 := c.errs[i]
			c2 := c.errs[j]
			err1 := c1.Err
			err2 := c2.Err
			if rootErr := commonRootError(err1, err2); rootErr != nil {
				c.addCommonError(rootErr, c1, c2)
			}
		}
	}
}
