// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buildkit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/solver/pb"
	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"go.opentelemetry.io/otel/trace"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/wscontents"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/go-ids"
)

const buildkitIntegrationVersion = 1

type baseRequest[V any] struct {
	sourceLabel    string             // For description purposes only, does not affect output.
	sourcePackage  schema.PackageName // For description purposes only, does not affect output.
	config         cfg.Configuration  // Doesn't affect the output.
	targetPlatform specs.Platform
	req            compute.Computable[*frontendReq]
	localDirs      []LocalContents // If set, the output is not cachable by us.

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
				Computable(fmt.Sprintf("local%d:contents", k), local.Module.VersionedFS(local.Path, local.ObserveChanges)).
				Str(fmt.Sprintf("local%d:path", k), local.Path)
		}
	} else {
		// We compute the digest so that the compute graph can dedup this build
		// with others that may be happening concurrently.
		for _, local := range l.localDirs {
			in = in.Marshal(fmt.Sprintf("local-contents:%s:%s", local.Module.Abs(), local.Path), func(ctx context.Context, w io.Writer) error {
				contents, err := compute.GetValue(ctx, local.Module.VersionedFS(local.Path, local.ObserveChanges))
				if err != nil {
					return err
				}

				digest, err := contents.ComputeDigest(ctx)
				if err != nil {
					return err
				}

				fmt.Fprintf(w, "%s\n", digest)
				return nil
			})
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
		"frontend":    req.Frontend,
		"frontendOpt": req.FrontendOpt,
		"ops":         ops,
		"inputs":      inputs,
	})
}

func (l *baseRequest[V]) Output() compute.Output {
	return compute.Output{
		// Because the localDirs' contents are not accounted for, assume the output is not stable.
		NonDeterministic: len(l.localDirs) > 0,
	}
}

func (l *baseRequest[V]) solve(ctx context.Context, deps compute.Resolved, keychain oci.Keychain, exp exporter[V]) (V, error) {
	var res V

	req := compute.MustGetDepValue(deps, l.req, "req")

	c, err := compute.GetValue(ctx, connectToClient(l.config, l.targetPlatform))
	if err != nil {
		return res, err
	}

	sid := ids.NewRandomBase62ID(8)

	attachables, err := prepareSession(ctx, keychain)
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
		FrontendAttrs:  req.FrontendOpt,
		FrontendInputs: req.FrontendInputs,
	}

	if len(l.localDirs) > 0 {
		solveOpt.LocalDirs = map[string]string{}
		for k, p := range l.localDirs {
			if !PreDigestLocalInputs {
				ws, ok := compute.GetDepWithType[wscontents.Versioned](deps, fmt.Sprintf("local%d:contents", k))
				if !ok {
					return res, fnerrors.InternalError("expected local contents to have been computed")
				}

				totalSize, err := fnfs.TotalSize(ctx, ws.Value.FS())
				if err != nil {
					fmt.Fprintln(console.Warnings(ctx), "Failed to estimate workspace size:", err)
				} else if totalSize > maxExpectedWorkspaceSize && !SkipExpectedMaxWorkspaceSizeCheck {
					return res, reportWorkspaceSizeErr(ctx, ws.Value.FS(), totalSize)
				}
			}

			solveOpt.LocalDirs[p.Name()] = filepath.Join(p.Module.Abs(), p.Path)
		}
	}

	fillInCaching(&solveOpt)

	ch := make(chan *client.SolveStatus)

	eg := executor.New(ctx, "buildkit.solve")

	var solveRes *client.SolveResponse
	eg.Go(func(ctx context.Context) error {
		// XXX Span data coming out from buildkit is not really working.
		ctx = trace.ContextWithSpan(ctx, nil)

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

	return exp.Provide(ctx, solveRes)
}
