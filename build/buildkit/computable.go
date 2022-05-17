// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buildkit

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"go.opentelemetry.io/otel/trace"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/consolesink"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/wscontents"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/foundation/workspace/tasks"
	"namespacelabs.dev/go-ids"
)

const maxExpectedWorkspaceSize uint64 = 32 * 1024 * 1024 // 32MB should be enough for everyone.

var SkipExpectedMaxWorkspaceSizeCheck = false

type LocalContents struct {
	Module         build.Workspace
	Path           string
	ObserveChanges bool
}

func (l LocalContents) Name() string {
	return filepath.Join(l.Module.ModuleName(), l.Path)
}

func DefinitionToImage(env ops.Environment, platform *specs.Platform, def *llb.Definition) compute.Computable[oci.Image] {
	return makeImage(env, platform, &frontendReq{Def: def}, nil, nil)
}

func LLBToImage(ctx context.Context, env ops.Environment, conf build.BuildTarget, state llb.State, localDirs ...LocalContents) (compute.Computable[oci.Image], error) {
	serialized, err := state.Marshal(ctx)
	if err != nil {
		return nil, err
	}

	return makeImage(env, conf.TargetPlatform(), &frontendReq{Def: serialized}, localDirs, conf.PublishName()), nil
}

func LLBToFS(ctx context.Context, env ops.Environment, platform *specs.Platform, state llb.State, localDirs ...LocalContents) (compute.Computable[fs.FS], error) {
	serialized, err := state.Marshal(ctx)
	if err != nil {
		return nil, err
	}

	base := reqBase{devHost: env.DevHost(), targetPlatform: platformOrDefault(platform), req: &frontendReq{Def: serialized}, localDirs: localDirs}
	return &reqToFS{reqBase: base}, nil
}

type reqBase struct {
	devHost        *schema.DevHost // Doesn't affect the output.
	targetPlatform specs.Platform
	req            *frontendReq
	localDirs      []LocalContents // If set, the output is not cachable by us.
}

func makeImage(env ops.Environment, platform *specs.Platform, req *frontendReq, localDirs []LocalContents, targetName compute.Computable[oci.AllocatedName]) compute.Computable[oci.Image] {
	base := reqBase{devHost: env.DevHost(), targetPlatform: platformOrDefault(platform), req: req, localDirs: localDirs}
	return &reqToImage{reqBase: base, targetName: targetName}
}

func platformOrDefault(targetPlatform *specs.Platform) specs.Platform {
	if targetPlatform == nil {
		return HostPlatform()
	}

	return *targetPlatform
}

type keyValue struct {
	Name  string
	Value *llb.Definition
}

func (l reqBase) buildInputs() *compute.In {
	in := compute.Inputs().
		Str("frontend", l.req.Frontend).
		StrMap("frontendOpts", l.req.FrontendOpt)

	for k, local := range l.localDirs {
		in = in.
			Computable(fmt.Sprintf("local%d:contents", k), local.Module.VersionedFS(local.Path, local.ObserveChanges)).
			Str(fmt.Sprintf("local%d:path", k), local.Path)
	}

	return in.Marshal("states", func(ctx context.Context, w io.Writer) error {
		var kvs []keyValue
		for k, v := range l.req.FrontendInputs {
			def, err := v.Marshal(ctx)
			if err != nil {
				return err
			}
			kvs = append(kvs, keyValue{k, def})
		}

		// Make order stable.
		sort.Slice(kvs, func(i, j int) bool {
			return strings.Compare(kvs[i].Name, kvs[j].Name) < 0
		})

		for _, kv := range kvs {
			if _, err := fmt.Fprintf(w, "%s:", kv.Name); err != nil {
				return err
			}
			if err := llb.WriteTo(kv.Value, w); err != nil {
				return err
			}
		}

		if l.req.Def == nil {
			return nil
		}

		return llb.WriteTo(l.req.Def, w)
	})
}

func (l reqBase) buildOutput() compute.Output {
	return compute.Output{
		// Because the localDirs' contents are not accounted for, assume the output is not stable.
		NonDeterministic: len(l.localDirs) > 0,
	}
}

type reqToImage struct {
	reqBase

	// If set, targetName will resolve into the allocated image name that this
	// image will be uploaded to, by our caller.
	targetName compute.Computable[oci.AllocatedName]

	compute.LocalScoped[oci.Image]
}

func (l *reqToImage) Action() *tasks.ActionEvent {
	return tasks.Action("buildkit.build-image").
		Arg("platform", devhost.FormatPlatform(l.targetPlatform)).
		WellKnown(tasks.WkCategory, "build").
		WellKnown(tasks.WkRuntime, "docker")
}

func (l *reqToImage) Inputs() *compute.In {
	return l.buildInputs()
}

func (l *reqToImage) Output() compute.Output {
	return l.buildOutput()
}

func (l *reqToImage) ImageRef() string { return "(buildkit)" } // Implements HasImageRef

func (l *reqToImage) Compute(ctx context.Context, deps compute.Resolved) (oci.Image, error) {
	// TargetName is not added as a dependency of the `reqToImage` compute node, or
	// our inputs are not stable.
	if l.targetName != nil {
		v, err := compute.GetValue(ctx, l.targetName)
		if err != nil {
			return nil, err
		}

		// If the target needs permissions, we don't do the direct push
		// optimization as we don't yet wire the keychain into buildkit.
		if v.Keychain == nil {
			i, err := solve(ctx, deps, l.reqBase, exportToRegistry(v.Repository, v.InsecureRegistry))
			if err != nil {
				bufNames := tasks.GetErrContext(ctx).GetBufNames()
				shouldLog := func() bool {
					sink := tasks.SinkFrom(ctx)
					if sink == nil {
						return false
					}
					consoleSink, ok := sink.(*consolesink.ConsoleSink)
					if !ok {
						return false
					}
					return !consoleSink.RecentInputSourcesContain(tasks.Attachments(ctx).ActionID())
				}
				for i := range bufNames {
					err = fnerrors.WithLogs(
						err,
						shouldLog,
						func() io.Reader {
							return tasks.Attachments(ctx).ReaderByOutputName(bufNames[len(bufNames)-i-1])
						})
					// TODO: allow multi buffer as contexts.
					break
				}
			}
			return i, err
		}
	}

	return solve(ctx, deps, l.reqBase, exportToImage())
}

type reqToFS struct {
	reqBase
	compute.LocalScoped[fs.FS]
}

func (l *reqToFS) Action() *tasks.ActionEvent {
	return tasks.Action("buildkit.build-fs").Arg("platform", devhost.FormatPlatform(l.targetPlatform)).WellKnown(tasks.WkAction, "build")
}
func (l *reqToFS) Inputs() *compute.In {
	return l.buildInputs()
}

func (l *reqToFS) Output() compute.Output {
	return l.buildOutput()
}

func (l *reqToFS) Compute(ctx context.Context, deps compute.Resolved) (fs.FS, error) {
	return solve(ctx, deps, l.reqBase, exportToFS())
}

func solve[V any](ctx context.Context, deps compute.Resolved, l reqBase, e exporter[V]) (V, error) {
	var res V

	c, err := compute.GetValue(ctx, connectToClient(l.devHost, l.targetPlatform))
	if err != nil {
		return res, err
	}

	sid := ids.NewRandomBase62ID(8)

	attachables, err := prepareSession(ctx)
	if err != nil {
		return res, err
	}

	if err := e.Prepare(ctx); err != nil {
		return res, err
	}

	solveOpt := client.SolveOpt{
		Session:        attachables,
		Exports:        e.Exports(),
		Frontend:       l.req.Frontend,
		FrontendAttrs:  l.req.FrontendOpt,
		FrontendInputs: l.req.FrontendInputs,
	}

	if len(l.localDirs) > 0 {
		solveOpt.LocalDirs = map[string]string{}
		for k, p := range l.localDirs {
			ws, ok := compute.GetDepWithType[wscontents.Versioned](deps, fmt.Sprintf("local%d:contents", k))
			if !ok {
				return res, fnerrors.InternalError("expected local contents to have been computed")
			}

			totalSize, err := fnfs.TotalSize(ctx, ws.Value.FS())
			if err != nil {
				fmt.Fprintln(console.Warnings(ctx), "Failed to estimate workspace size:", err)
			} else if totalSize > maxExpectedWorkspaceSize && !SkipExpectedMaxWorkspaceSizeCheck {
				return res, fnerrors.InternalError(`the workspace snapshot is unexpectedly large (%s vs max expected %s);
this is likely a problem with the way that foundation is filtering workspace contents.

If you don't think this is an actual issue, please re-run with --skip_buildkit_workspace_size_check=true.`,
					humanize.Bytes(totalSize), humanize.Bytes(maxExpectedWorkspaceSize))
			}

			solveOpt.LocalDirs[p.Name()] = filepath.Join(p.Module.Abs(), p.Path)
		}
	}

	fillInCaching(&solveOpt)

	ch := make(chan *client.SolveStatus)

	eg, wait := executor.New(ctx)

	var solveRes *client.SolveResponse
	eg.Go(func(ctx context.Context) error {
		// XXX Span data coming out from buildkit is not really working.
		ctx = trace.ContextWithSpan(ctx, nil)

		var err error
		solveRes, err = c.Solve(ctx, l.req.Def, solveOpt, ch)
		return err
	})

	setupOutput(ctx, sid, eg, ch)

	if err := wait(); err != nil {
		return res, err
	}

	return e.Provide(ctx, solveRes)
}

type exporter[V any] interface {
	Prepare(context.Context) error
	Exports() []client.ExportEntry
	Provide(context.Context, *client.SolveResponse) (V, error)
}

func exportToImage() exporter[oci.Image] { return &exportImage{} }

type exportImage struct {
	output *os.File
}

func (e *exportImage) Prepare(ctx context.Context) error {
	f, err := dirs.CreateUserTemp("buildkit", "image")
	if err != nil {
		return err
	}

	// ExportEntry below takes care of closing f.
	e.output = f

	compute.On(ctx).Cleanup(tasks.Action("buildkit.build-image.cleanup"), func(ctx context.Context) error {
		return os.Remove(f.Name())
	})

	return nil
}

func (e *exportImage) Exports() []client.ExportEntry {
	return []client.ExportEntry{{
		Type: client.ExporterDocker,
		Output: func(_ map[string]string) (io.WriteCloser, error) {
			return e.output, nil
		},
	}}
}

func (e *exportImage) Provide(ctx context.Context, _ *client.SolveResponse) (oci.Image, error) {
	return IngestFromFS(ctx, fnfs.Local(filepath.Dir(e.output.Name())), filepath.Base(e.output.Name()), false)
}

func IngestFromFS(ctx context.Context, fsys fs.FS, path string, compressed bool) (oci.Image, error) {
	img, err := tarball.Image(func() (io.ReadCloser, error) {
		f, err := fsys.Open(path)
		if err != nil {
			return nil, err
		}

		fi, err := f.Stat()
		if err != nil {
			return nil, fnerrors.InternalError("failed to stat intermediate image: %w", err)
		}

		progress := artifacts.NewProgressReader(f, uint64(fi.Size()))
		tasks.Attachments(ctx).SetProgress(progress)

		if !compressed {
			return progress, nil
		}

		gr, err := gzip.NewReader(progress)
		if err != nil {
			return nil, err
		}

		return andClose{gr, progress}, nil
	}, nil)
	if err != nil {
		return nil, err
	}

	return attachImageResults(ctx, img)
}

func attachImageResults(ctx context.Context, img oci.Image) (oci.Image, error) {
	digest, err := img.Digest()
	if err != nil {
		return nil, err
	}

	cfgName, err := img.ConfigName()
	if err != nil {
		return nil, err
	}

	tasks.Attachments(ctx).AddResult("digest", digest).AddResult("config", cfgName)
	return img, nil
}

type andClose struct {
	actual io.ReadCloser
	closer io.Closer
}

func (a andClose) Read(p []byte) (int, error) { return a.actual.Read(p) }
func (a andClose) Close() error {
	err := a.actual.Close()
	ioerr := a.closer.Close()
	if err != nil {
		return err
	}
	return ioerr
}

func exportToFS() exporter[fs.FS] { return &exportFS{} }

type exportFS struct {
	outputDir string
}

func (e *exportFS) Prepare(ctx context.Context) error {
	dir, err := dirs.CreateUserTempDir("buildkit", "fs")
	if err != nil {
		return err
	}

	e.outputDir = dir

	compute.On(ctx).Cleanup(tasks.Action("buildkit.build-fs.cleanup"), func(ctx context.Context) error {
		return os.RemoveAll(dir)
	})

	return nil
}

func (e *exportFS) Exports() []client.ExportEntry {
	return []client.ExportEntry{{
		Type:      client.ExporterLocal,
		OutputDir: e.outputDir,
	}}
}

func (e *exportFS) Provide(context.Context, *client.SolveResponse) (fs.FS, error) {
	return fnfs.Local(e.outputDir), nil
}

func exportToRegistry(ref string, insecure bool) exporter[oci.Image] {
	return &exportRegistry{ref: ref, insecure: insecure}
}

type exportRegistry struct {
	ref      string
	insecure bool

	parsed name.Repository
}

func (e *exportRegistry) Prepare(ctx context.Context) error {
	p, err := name.NewRepository(e.ref, e.nameOpts()...)
	if err != nil {
		return err
	}
	e.parsed = p
	return nil
}

func (e *exportRegistry) nameOpts() []name.Option {
	if e.insecure {
		return []name.Option{name.Insecure}
	}

	return nil
}

func (e *exportRegistry) Exports() []client.ExportEntry {
	return []client.ExportEntry{{
		Type: client.ExporterImage,
		Attrs: map[string]string{
			"push":              "true",
			"name":              e.ref,
			"push-by-digest":    "true",
			"registry.insecure": fmt.Sprintf("%v", e.insecure),
		},
	}}
}

func (e *exportRegistry) Provide(ctx context.Context, res *client.SolveResponse) (oci.Image, error) {
	digest, ok := res.ExporterResponse[exptypes.ExporterImageDigestKey]
	if !ok {
		return nil, fnerrors.New("digest is missing from result")
	}

	p, err := name.NewDigest(e.parsed.Name()+"@"+digest, e.nameOpts()...)
	if err != nil {
		return nil, err
	}

	img, err := compute.WithGraphLifecycle(ctx, func(ctx context.Context) (oci.Image, error) {
		return remote.Image(p, remote.WithContext(ctx))
	})
	if err != nil {
		return nil, err
	}

	return attachImageResults(ctx, img)
}
