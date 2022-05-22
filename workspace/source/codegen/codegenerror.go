// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package codegen

import (
	"errors"
	"fmt"
	"io"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

type GenerateError struct {
	PackageName schema.PackageName
	What        string
	Err         error
}

func (g GenerateError) fingerprint() string {
	return fmt.Sprintf("%s:%v:%v", g.What, g.PackageName, g.Err)
}

type GenerateErrorAggregator struct {
	ErrChannel chan GenerateError
	flush      chan bool

	errs []GenerateError
}

func NewErrorAggregator() *GenerateErrorAggregator {
	return &GenerateErrorAggregator{
		ErrChannel: make(chan GenerateError),
		flush:      make(chan bool),
	}
}

// Start method starts the run loop for the aggregator, which is terminated
// when flushed.
func (g *GenerateErrorAggregator) Start() {
	go func() {
		for {
			select {
			case generateErr := <-g.ErrChannel:
				g.errs = append(g.errs, generateErr)

			case <-g.flush:
				return
			}
		}
	}()
}

// Flush signals the aggregator to stop listening for generate errors.
func (g *GenerateErrorAggregator) Flush(w io.Writer, colors bool) {
	g.flush <- true
	g.formatErrors(w, colors)
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

func (g *GenerateErrorAggregator) formatErrors(w io.Writer, colors bool) {
	genMultiErrs := newGenMultiErrs(g.errs)
	for i := 0; i < len(g.errs); i++ {
		for j := 0; j < len(g.errs); j++ {
			g1 := g.errs[i]
			g2 := g.errs[j]
			err1 := g1.Err
			err2 := g2.Err
			if rootErr := commonRootError(err1, err2); rootErr != nil {
				genMultiErrs.addCommonError(w, rootErr, g1, g2)
			}
		}
	}
	genMultiErrs.format(w, colors)
}

type packages map[string]struct{}

// Utility struct used for error aggregation and de-duplication.
type genMultiErrs struct {
	// all GenerateErrors emitted to the channel.
	generrs []GenerateError

	// aggregates GenerateErrors by their root cause.
	commonerrs map[error]map[string]packages

	// tracks duplicate GenerateErrors by fingerprint.
	duplicategenerrs map[string]struct{}
}

func newGenMultiErrs(generrs []GenerateError) *genMultiErrs {
	return &genMultiErrs{
		commonerrs:       make(map[error]map[string]packages),
		generrs:          generrs,
		duplicategenerrs: make(map[string]struct{}),
	}
}

func (g *genMultiErrs) addCommonError(w io.Writer, error error, err1 GenerateError, err2 GenerateError) {
	_, has := g.commonerrs[error]
	if !has {
		g.commonerrs[error] = make(map[string]packages)
	}
	whatpkgs := g.commonerrs[error]

	_, has = whatpkgs[err1.What]
	if !has {
		whatpkgs[err1.What] = make(packages)
	}
	pkgs := whatpkgs[err1.What]
	pkgs[err1.PackageName.String()] = struct{}{}

	_, has = whatpkgs[err2.What]
	if !has {
		whatpkgs[err2.What] = make(packages)
	}
	pkgs = whatpkgs[err2.What]
	pkgs[err2.PackageName.String()] = struct{}{}

	_, has = g.duplicategenerrs[err1.fingerprint()]
	if !has {
		g.duplicategenerrs[err1.fingerprint()] = struct{}{}
	}
	_, has = g.duplicategenerrs[err2.fingerprint()]
	if !has {
		g.duplicategenerrs[err2.fingerprint()] = struct{}{}
	}
}

func (g *genMultiErrs) format(w io.Writer, colors bool) {
	// Print aggregated errors.
	for commonErr, whatpkgs := range g.commonerrs {
		for what, pkgs := range whatpkgs {
			var pkgnames []string
			for p := range pkgs {
				pkgnames = append(pkgnames, p)
			}
			err := fnerrors.CodegenError(commonErr, what, pkgnames...)
			fnerrors.Format(w, err, fnerrors.WithColors(colors))
		}
	}
	// Print all unique GenerateError(s) that don't have a common root.
	var uniqgenerrs []GenerateError
	for _, generr := range g.generrs {
		_, duplicate := g.duplicategenerrs[generr.fingerprint()]
		if !duplicate {
			uniqgenerrs = append(uniqgenerrs, generr)
		}
	}
	for _, generr := range uniqgenerrs {
		err := fnerrors.CodegenError(generr.Err, generr.What, generr.PackageName.String())
		fnerrors.Format(w, err, fnerrors.WithColors(colors))
	}
}
