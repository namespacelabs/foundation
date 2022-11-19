// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/atomic"
)

var healthz struct {
	mu               sync.RWMutex
	liveNames        []string
	liveChecker      []Checker
	readinessNames   []string
	readinessChecker []Checker
}

const defaultProbeTimeout = 1 * time.Second

type Checker interface {
	Check(context.Context) error

	isManual() bool
}

type CheckerFunc func(context.Context) error

func (c CheckerFunc) Check(ctx context.Context) error { return c(ctx) }
func (c CheckerFunc) isManual() bool                  { return false }

var shutdownStarted = atomic.NewBool(false)

func init() {
	registerReadiness("shutdown-requested", CheckerFunc(func(ctx context.Context) error {
		if shutdownStarted.Load() {
			return errors.New("shutdown requested")
		}
		return nil
	}))
}

func MarkShutdownStarted() {
	shutdownStarted.Store(true)
}

type memoizingChecker struct {
	checker   Checker
	succeeded atomic.Bool
}

func (m *memoizingChecker) Check(ctx context.Context) error {
	if m.succeeded.Load() {
		return nil
	}

	err := m.checker.Check(ctx)
	if err == nil {
		m.succeeded.Store(true)
	}
	return err
}

func (m *memoizingChecker) isManual() bool { return false }

func registerLiveness(name string, checker Checker) {
	healthz.mu.Lock()
	healthz.liveNames = append(healthz.liveNames, name)
	healthz.liveChecker = append(healthz.liveChecker, checker)
	healthz.mu.Unlock()
}

func registerReadiness(name string, checker Checker) {
	var actualChecker Checker
	if checker.isManual() {
		actualChecker = checker
	} else {
		actualChecker = &memoizingChecker{checker: checker}
	}

	healthz.mu.Lock()
	healthz.readinessNames = append(healthz.readinessNames, name)
	healthz.readinessChecker = append(healthz.readinessChecker, actualChecker)
	healthz.mu.Unlock()
}

func livezEndpoint() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		healthz.mu.RLock()
		checkers := make([]Checker, len(healthz.liveChecker))
		copy(checkers, healthz.liveChecker)
		names := make([]string, len(healthz.liveNames))
		copy(names, healthz.liveNames)
		healthz.mu.RUnlock()

		// Run checks on a copy to guarantee we never block other /livez or /readyz calls.
		runChecks(rw, r, names, checkers)
	})
}

func readyzEndpoint() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		healthz.mu.RLock()
		checkers := make([]Checker, len(healthz.readinessChecker))
		copy(checkers, healthz.readinessChecker)
		names := make([]string, len(healthz.readinessNames))
		copy(names, healthz.readinessNames)
		healthz.mu.RUnlock()

		// Run checks on a copy to guarantee we never block other /livez or /readyz calls.
		runChecks(rw, r, names, checkers)
	})
}

func runChecks(rw http.ResponseWriter, r *http.Request, names []string, checkers []Checker) {
	ctx, done := context.WithTimeout(r.Context(), defaultProbeTimeout)
	defer done()

	errs := make([]error, len(checkers))
	errCount := 0
	for k, checker := range checkers {
		// XXX guard against panic?
		errs[k] = checker.Check(ctx)
		if errs[k] != nil {
			errCount++
		}
	}

	if errCount > 0 {
		rw.WriteHeader(500)
		fmt.Fprintf(rw, "%d failures in %d checks\n\n", errCount, len(errs))
	} else {
		rw.WriteHeader(200)
		fmt.Fprintf(rw, "All OK\n\n")
	}

	for k, name := range names {
		if errs[k] == nil {
			fmt.Fprintf(rw, "%s: OK\n", name)
		} else {
			fmt.Fprintf(rw, "%s: failed: %v", name, errs[k])
		}
	}
}
