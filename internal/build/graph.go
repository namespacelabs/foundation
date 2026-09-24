// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package build

import (
	"context"
	"fmt"
	"sync"
)

// GraphPass collects build nodes during graph generation and rewrites them into
// executable compute nodes when graph generation is complete.
type GraphPass interface {
	Finalize() error
}

// Graph coordinates build-system-specific graph generation passes.
type Graph struct {
	mu        sync.Mutex
	passes    map[string]GraphPass
	order     []GraphPass
	finalized bool
}

func NewGraph() *Graph {
	return &Graph{passes: map[string]GraphPass{}}
}

// Pass returns the pass registered under name, creating it if necessary.
func (g *Graph) Pass(name string, create func() GraphPass) (GraphPass, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.finalized {
		return nil, fmt.Errorf("build graph has already been finalized")
	}
	if pass := g.passes[name]; pass != nil {
		return pass, nil
	}
	pass := create()
	g.passes[name] = pass
	g.order = append(g.order, pass)
	return pass, nil
}

// Finalize runs each registered graph pass after graph generation is complete.
func (g *Graph) Finalize() error {
	g.mu.Lock()
	if g.finalized {
		g.mu.Unlock()
		return fmt.Errorf("build graph has already been finalized")
	}
	g.finalized = true
	passes := append([]GraphPass(nil), g.order...)
	g.mu.Unlock()

	for _, pass := range passes {
		if err := pass.Finalize(); err != nil {
			return err
		}
	}
	return nil
}

type graphContextKey struct{}

func WithGraph(ctx context.Context, graph *Graph) context.Context {
	return context.WithValue(ctx, graphContextKey{}, graph)
}

func GraphFromContext(ctx context.Context) (*Graph, bool) {
	graph, ok := ctx.Value(graphContextKey{}).(*Graph)
	return graph, ok
}
