// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package source

import (
	"context"
	"encoding/json"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rs/zerolog"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/artifacts/fsops"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/sdk/buf"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/foundation/workspace/source/protos"
	"namespacelabs.dev/foundation/workspace/tasks"
	"tailscale.com/util/multierr"
)

type GoProtosOpts struct {
	HTTPGateway bool
	Framework   OpProtoGen_Framework
}

func RegisterGraphHandlers() {
	ops.Register[*OpProtoGen](statefulGen{})
}

type statefulGen struct{}

var _ ops.BatchedDispatcher[*OpProtoGen] = statefulGen{}

func (statefulGen) Handle(ctx context.Context, env ops.Environment, _ *schema.Definition, msg *OpProtoGen) (*ops.HandleResult, error) {
	wenv, ok := env.(workspace.MutableWorkspaceEnvironment)
	if !ok {
		return nil, fnerrors.New("WorkspaceEnvironment required")
	}

	mod := &perModuleGen{}

	if msg.GenerateHttpGateway {
		mod.withHTTP.add(msg.Framework, msg.Protos)
	} else {
		mod.withoutHTTP.add(msg.Framework, msg.Protos)
	}

	return nil, generateProtoSrcs(ctx, buf.Image(ctx, env, wenv), mod, wenv.OutputFS())
}

func (statefulGen) StartSession(ctx context.Context, env ops.Environment) ops.Session[*OpProtoGen] {
	wenv, ok := env.(workspace.MutableWorkspaceEnvironment)
	if !ok {
		// An error will then be returned in Close().
		wenv = nil
	}

	return &multiGen{ctx: ctx, buf: buf.Image(ctx, env, wenv), wenv: wenv}
}

type multiGen struct {
	ctx  context.Context
	buf  compute.Computable[oci.Image]
	wenv workspace.MutableWorkspaceEnvironment

	mu    sync.Mutex
	locs  []workspace.Location
	opts  []GoProtosOpts
	files []*protos.FileDescriptorSetAndDeps
}

func (m *multiGen) Handle(ctx context.Context, env ops.Environment, _ *schema.Definition, msg *OpProtoGen) (*ops.HandleResult, error) {
	wenv, ok := env.(workspace.Packages)
	if !ok {
		return nil, fnerrors.New("workspace.Packages required")
	}

	loc, err := wenv.Resolve(ctx, schema.PackageName(msg.PackageName))
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.locs = append(m.locs, loc)
	m.opts = append(m.opts, GoProtosOpts{
		HTTPGateway: msg.GenerateHttpGateway,
		Framework:   msg.Framework,
	})
	m.files = append(m.files, msg.Protos)

	return nil, nil
}

type perLanguageDescriptors struct {
	descriptorsMap map[OpProtoGen_Framework][]*protos.FileDescriptorSetAndDeps
}

func (p *perLanguageDescriptors) add(framework OpProtoGen_Framework, fileDescSet *protos.FileDescriptorSetAndDeps) {
	if p.descriptorsMap == nil {
		p.descriptorsMap = map[OpProtoGen_Framework][]*protos.FileDescriptorSetAndDeps{}
	}

	descriptors, ok := p.descriptorsMap[framework]
	if !ok {
		descriptors = []*protos.FileDescriptorSetAndDeps{}
	}

	descriptors = append(descriptors, fileDescSet)
	p.descriptorsMap[framework] = descriptors
}

type perModuleGen struct {
	root        *workspace.Module
	withHTTP    perLanguageDescriptors
	withoutHTTP perLanguageDescriptors
}

func ensurePerModule(mods []*perModuleGen, root *workspace.Module) ([]*perModuleGen, *perModuleGen) {
	for _, mod := range mods {
		if mod.root.Abs() == root.Abs() {
			return mods, mod
		}
	}

	mod := &perModuleGen{root: root}
	return append(mods, mod), mod
}

func (m *multiGen) Commit() error {
	if m.wenv == nil {
		return fnerrors.New("WorkspaceEnvironment required")
	}

	var mods []*perModuleGen
	var mod *perModuleGen

	m.mu.Lock()

	for k := range m.locs {
		mods, mod = ensurePerModule(mods, m.locs[k].Module)

		if m.opts[k].HTTPGateway {
			mod.withHTTP.add(m.opts[k].Framework, m.files[k])
		} else {
			mod.withoutHTTP.add(m.opts[k].Framework, m.files[k])
		}
	}

	m.mu.Unlock()

	var errs []error
	for _, mod := range mods {
		if err := generateProtoSrcs(m.ctx, m.buf, mod, m.wenv.OutputFS()); err != nil {
			errs = append(errs, err)
		}
	}

	return multierr.New(errs...)
}

func makeProtoSrcs(buf compute.Computable[oci.Image], parsed *protos.FileDescriptorSetAndDeps, opts GoProtosOpts) compute.Computable[fs.FS] {
	return &genGoProtosAtLoc{
		buf:         buf,
		fileDescSet: parsed,
		opts:        opts,
	}
}

func generateProtoSrcs(ctx context.Context, buf compute.Computable[oci.Image], mod *perModuleGen, out fnfs.ReadWriteFS) error {
	var fsys []compute.Computable[fs.FS]

	for framework, descriptors := range mod.withHTTP.descriptorsMap {
		if len(descriptors) != 0 {
			fsys = append(fsys, makeProtoSrcs(buf, protos.Merge(descriptors...),
				GoProtosOpts{HTTPGateway: true, Framework: framework}))
		}
	}

	for framework, descriptors := range mod.withoutHTTP.descriptorsMap {
		if len(descriptors) != 0 {
			fsys = append(fsys, makeProtoSrcs(buf, protos.Merge(descriptors...),
				GoProtosOpts{HTTPGateway: false, Framework: framework}))
		}
	}

	if len(fsys) == 0 {
		return nil
	}

	merged, err := compute.Get(ctx, fsops.Merge(fsys))
	if err != nil {
		return err
	}

	if err := fnfs.WriteFSToWorkspace(ctx, console.Stdout(ctx), out, merged.Value); err != nil {
		return err
	}

	return nil
}

type genGoProtosAtLoc struct {
	buf         compute.Computable[oci.Image]
	fileDescSet *protos.FileDescriptorSetAndDeps
	opts        GoProtosOpts

	compute.LocalScoped[fs.FS]
}

var _ compute.Computable[fs.FS] = &genGoProtosAtLoc{}

func (g *genGoProtosAtLoc) Action() *tasks.ActionEvent {
	var files []string
	for _, fds := range g.fileDescSet.File {
		files = append(files, fds.GetName())
	}

	return tasks.Action("proto.generate").
		Arg("http_gateway", g.opts.HTTPGateway).
		Arg("framework", strings.ToLower(g.opts.Framework.String())).
		Arg("files", files)
}

func (g *genGoProtosAtLoc) Inputs() *compute.In {
	return compute.Inputs().Computable("buf", g.buf).Proto("filedescset", g.fileDescSet).JSON("opts", g.opts)
}

func (g *genGoProtosAtLoc) Compute(ctx context.Context, deps compute.Resolved) (fs.FS, error) {
	// These directories are used within the container. Both will be mapped to temp dirs in the host
	// which are managed below, and will be deleted on completion.
	const outDir = "/out"
	const srcDir = "/src"

	t := buf.GenerateTmpl{
		Version: "v1",
	}

	if g.opts.Framework == OpProtoGen_GO {
		t.Plugins = append(t.Plugins,
			buf.PluginTmpl{Name: "go", Out: outDir, Opt: []string{"paths=source_relative"}},
			buf.PluginTmpl{Name: "go-grpc", Out: outDir, Opt: []string{"paths=source_relative", "require_unimplemented_servers=false"}})
	}

	if g.opts.Framework == OpProtoGen_TYPESCRIPT {
		// We could also use remote plugins (buf.build/protocolbuffers/plugins/js, buf.build/grpc/plugins/node)
		// but the build time would likely be higher and the builds wouldn't be hermetic.

		// Generates "_pb.js" file
		t.Plugins = append(t.Plugins,
			buf.PluginTmpl{Name: "js", Out: outDir, Opt: []string{"import_style=commonjs,binary"}})
		// Generates "_grpc_pb.js" file
		// TODO(@nicolasalt): switch to a local plugin from `grpc-tools` for speed and making the build hermetic.
		// I couldn't make it work.
		t.Plugins = append(t.Plugins,
			buf.PluginTmpl{Remote: "buf.build/grpc/plugins/node:v1.11.2-1", Out: outDir, Opt: []string{"grpc_js"}})
		// Generates "_pb.d.ts" and "_grpc_pb.d.ts" files
		// TODO(Nik): look into "grpc_tools_node_protoc_ts" if these "_grpc_" files are not fully
		// compatible with the JS generated by the grpc plugin.
		t.Plugins = append(t.Plugins,
			buf.PluginTmpl{Name: "ts", Out: outDir, Opt: []string{"service=grpc-node", "mode=grpc-js"}})
	}

	if g.opts.HTTPGateway {
		t.Plugins = append(t.Plugins, buf.PluginTmpl{
			Name: "grpc-gateway",
			Out:  outDir,
			Opt:  []string{"paths=source_relative", "generate_unbound_methods=true"},
		})
	}

	templateBytes, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}

	genprotoSrc, err := dirs.CreateUserTempDir("genproto", "src")
	if err != nil {
		return nil, err
	}

	defer os.RemoveAll(genprotoSrc)

	// Make all files available to buf, but then constrain below which files we request
	// generation on.
	fdsBytes, err := proto.Marshal(g.fileDescSet.AsFileDescriptorSet())
	if err != nil {
		return nil, err
	}

	const srcfile = "image.bin"
	srcPath := filepath.Join(genprotoSrc, srcfile)
	if err := ioutil.WriteFile(srcPath, fdsBytes, 0600); err != nil {
		return nil, err
	}

	args := []string{"generate", "--template", string(templateBytes), srcfile}

	for _, p := range g.fileDescSet.File {
		args = append(args, "--path", p.GetName())
	}

	// The strategy here is to produce all results onto a directory structure that mimics the workspace,
	// but to a location off-workspace. This allow us to read the results into a snapshot without modifying
	// the workspace in-place. We can then decide to commit those results to the workspace.

	targetDir, err := dirs.CreateUserTempDir("genproto", "filedescset")
	if err != nil {
		return nil, err
	}

	bufimg := compute.GetDepValue(deps, g.buf, "buf")

	mounts := []*rtypes.LocalMapping{
		{HostPath: genprotoSrc, ContainerPath: srcDir},
		{HostPath: targetDir, ContainerPath: outDir},
	}

	out := console.Output(ctx, "buf")
	if err := buf.Run(ctx, bufimg, rtypes.IO{Stdout: out, Stderr: out}, srcDir, mounts, args); err != nil {
		return nil, err
	}

	result := fnfs.Local(targetDir)

	// Only initiate a cleanup after we're done compiling.
	compute.On(ctx).Cleanup(tasks.Action("proto.generate.cleanup"), func(ctx context.Context) error {
		if err := os.RemoveAll(targetDir); err != nil {
			zerolog.Ctx(ctx).Warn().Err(err).Msg("failed to cleanup target dir")
		}
		return nil // Never fail.
	})

	return result, nil
}

func GenGoProtosAtPaths(ctx context.Context, env ops.Environment, loader workspace.Packages, src fs.FS, opts GoProtosOpts, paths []string, out fnfs.ReadWriteFS) error {
	parsed, err := protos.Parse(src, paths)
	if err != nil {
		return err
	}

	mod := &perModuleGen{}
	if opts.HTTPGateway {
		mod.withHTTP.add(opts.Framework, parsed)
	} else {
		mod.withoutHTTP.add(opts.Framework, parsed)
	}

	return generateProtoSrcs(ctx, buf.Image(ctx, env, loader), mod, out)
}
