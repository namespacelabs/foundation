// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/containerd/containerd/content"
	"github.com/dustin/go-humanize"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/patternmatcher"
	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/framework/secrets"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/go-ids"
)

const buildkitIntegrationVersion = 1

type baseRequest[V any] struct {
	sourceLabel    string             // For description purposes only, does not affect output.
	sourcePackage  schema.PackageName // For description purposes only, does not affect output.
	makeClient     ClientFactory      // Doesn't affect the output.
	targetPlatform *specs.Platform    // If one is set, may be used to select the target build cluster.
	req            compute.Computable[*FrontendRequest]
	localDirs      []LocalContents // If set, the output is not cachable by us.
	secrets        secrets.GroundedSecrets

	compute.LocalScoped[V]
}

func (l *baseRequest[V]) Inputs() *compute.In {
	in := compute.Inputs().
		JSON("version", buildkitIntegrationVersion).
		Computable("req", l.req)

	if !PreDigestLocalInputs {
		// Local contents are added as dependencies to trigger continuous builds.
		for k, local := range l.localDirs {
			in = in.
				Computable(fmt.Sprintf("local%d:contents", k), memfs.DeferSnapshot(local.Module.ReadOnlyFS(local.Path), memfs.SnapshotOpts{
					ExcludePatterns: MakeLocalExcludes(local),
				})).
				Str(fmt.Sprintf("local%d:path", k), local.Path)
		}
	} else if len(l.localDirs) > 0 {
		in = in.Indigestible("localDirs", "not cacheable")
	}

	for k, local := range l.localDirs {
		if trigger := local.Module.ChangeTrigger(local.Path, local.ExcludePatterns); trigger != nil {
			in = in.Computable(fmt.Sprintf("trigger:%d", k), trigger)
		}
	}

	return in
}

type keyValue struct {
	Name  string
	Value *llb.Definition
}

type explainEntity struct {
	Op         pb.Op
	Digest     digest.Digest
	OpMetadata pb.OpMetadata
}

type explainInput struct {
	Name string
	Ops  []explainEntity
}

// Implements the explain protocol.
func (l *baseRequest[V]) Explain(ctx context.Context, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	var ops []explainEntity
	var inputs []explainInput

	toOp := func(def *llb.Definition) ([]explainEntity, error) {
		var ents []explainEntity
		for _, dt := range def.Def {
			op := &pb.Op{}
			if err := op.Unmarshal(dt); err != nil {
				return nil, fnerrors.New("failed to parse op: %w", err)
			}

			digest := digest.FromBytes(dt)
			ents = append(ents, explainEntity{Op: *op, Digest: digest, OpMetadata: def.Metadata[digest]})
		}
		return ents, nil
	}

	req, err := compute.GetValue(ctx, l.req)
	if err != nil {
		return err
	}

	if def := req.Def; def != nil {
		var err error
		ops, err = toOp(def)
		if err != nil {
			return err
		}
	}

	for k, v := range req.FrontendInputs {
		def, err := v.Marshal(ctx)
		if err != nil {
			return err
		}

		ops, err := toOp(def)
		if err != nil {
			return err
		}

		inputs = append(inputs, explainInput{Name: k, Ops: ops})
	}

	return enc.Encode(map[string]interface{}{
		"frontend":      req.Frontend,
		"frontendAttrs": req.FrontendAttrs,
		"ops":           ops,
		"inputs":        inputs,
	})
}

func (l *baseRequest[V]) Output() compute.Output {
	return compute.Output{
		// Because the localDirs' contents are not accounted for, assume the output is not stable.
		NonDeterministic: len(l.localDirs) > 0,
	}
}

func (l *baseRequest[V]) solve(ctx context.Context, c *GatewayClient, deps compute.Resolved, keychain oci.Keychain, exp exporter[V]) (V, error) {
	var res V

	req := compute.MustGetDepValue(deps, l.req, "req")

	sid := ids.NewRandomBase62ID(8)

	attachables, err := prepareSession(ctx, keychain, l.secrets, req.Secrets)
	if err != nil {
		return res, err
	}

	if err := exp.Prepare(ctx); err != nil {
		return res, err
	}

	solveOpt := client.SolveOpt{
		Session:        attachables,
		Exports:        exp.Exports(),
		Frontend:       req.Frontend,
		FrontendAttrs:  req.FrontendAttrs,
		FrontendInputs: req.FrontendInputs,
	}

	var attrs []map[string]string
	for _, exp := range solveOpt.Exports {
		attrs = append(attrs, exp.Attrs)
	}

	fmt.Fprintf(console.Debug(ctx), "buildkit/%s: frontendAttrs: %v\n", sid, req.FrontendAttrs)
	fmt.Fprintf(console.Debug(ctx), "buildkit/%s: exports.attrs: %v\n", sid, attrs)

	if len(l.localDirs) > 0 {
		solveOpt.LocalDirs = map[string]string{}
		for _, local := range l.localDirs {
			solveOpt.LocalDirs[local.Abs()] = filepath.Join(local.Module.Abs(), local.Path)

			if !PreDigestLocalInputs {
				if SkipExpectedMaxWorkspaceSizeCheck {
					continue
				}

				matcher, err := patternmatcher.New(MakeLocalExcludes(local))
				if err != nil {
					return res, err
				}

				ws := local.Module.ReadOnlyFS(local.Path)
				w, err := reportWorkspaceSize(ctx, ws, matcher)
				if err != nil {
					return res, err
				}

				fmt.Fprintf(console.Debug(ctx), "buildkit.local: %s: total size: %v (%d files)\n", local.Abs(), humanize.Bytes(w.TotalSize), len(w.Files))

				if w.TotalSize > maxExpectedWorkspaceSize {
					return res, makeSizeError(w)
				}
			}
		}
	}

	solveOpt.OCIStores = map[string]content.Store{
		"cache": &cacheStore{compute.Cache(ctx)},
	}

	fillInCaching(&solveOpt)

	ch := make(chan *client.SolveStatus)

	eg := executor.New(ctx, "buildkit.solve")

	var solveRes *client.SolveResponse
	eg.Go(func(ctx context.Context) error {
		var err error
		solveRes, err = c.Solve(ctx, req.Def, solveOpt, ch)
		return err
	})

	logid := l.sourcePackage.String()
	if logid == "" {
		logid = l.sourceLabel
	}

	setupOutput(ctx, logid, sid, eg, ch)

	if err := eg.Wait(); err != nil {
		return res, err
	}

	fmt.Fprintf(console.Debug(ctx), "buildkit/%s: exported (%s): %v\n", sid, exp.Kind(), solveRes.ExporterResponse)

	return exp.Provide(ctx, solveRes, c.BuildkitOpts())
}
