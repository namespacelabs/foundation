// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package lsp

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/ast/astutil"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
)

type server struct {
	conn      jsonrpc2.Conn
	client    protocol.Client
	openFiles *OpenFiles
	log       io.Writer
	verbose   bool
}

func newServer(ctx context.Context, conn jsonrpc2.Conn, client protocol.Client) *server {
	return &server{conn, client, NewOpenFiles(), console.Stderr(ctx), true}
}

const generateCommandID = "ns_generate"

func (s *server) logf(format string, a ...any) {
	if !s.verbose {
		return
	}
	fmt.Fprintf(s.log, format, a...)
}

// Lifecycle

func (s *server) Initialize(ctx context.Context, params *protocol.InitializeParams) (result *protocol.InitializeResult, err error) {
	fmt.Fprintln(s.log, "Initialize")
	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			DocumentFormattingProvider: true,
			TextDocumentSync: &protocol.TextDocumentSyncOptions{
				OpenClose: true,
				Change:    protocol.TextDocumentSyncKindFull,
				Save:      &protocol.SaveOptions{},
			},
			ExecuteCommandProvider: &protocol.ExecuteCommandOptions{
				Commands: []string{generateCommandID},
			},
			CodeActionProvider: &protocol.CodeActionOptions{
				CodeActionKinds: []protocol.CodeActionKind{protocol.Source},
			},
			DefinitionProvider: &protocol.DefinitionOptions{},
			HoverProvider:      &protocol.HoverOptions{},
		},
	}, nil
}
func (s *server) Initialized(ctx context.Context, params *protocol.InitializedParams) (err error) {
	s.logf("Initialized\n")
	config, err := s.loadConfig(ctx)
	if err != nil {
		s.logf("Failed to load config\n")
		return err
	}
	s.verbose = config.traceServer != "" && config.traceServer != "off"
	return nil
}
func (s *server) Shutdown(ctx context.Context) (err error) {
	s.logf("Shutdown\n")
	return nil
}
func (s *server) Exit(ctx context.Context) (err error) {
	s.logf("Exit\n")
	s.conn.Close()
	return nil
}

// Document Sync

func (s *server) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) (err error) {
	if err := s.openFiles.DidOpen(params); err != nil {
		return err
	}

	return s.updateDiagnostics(ctx, protocol.VersionedTextDocumentIdentifier{
		TextDocumentIdentifier: protocol.TextDocumentIdentifier{
			URI: params.TextDocument.URI,
		},
		Version: params.TextDocument.Version,
	})
}
func (s *server) DidClose(ctx context.Context, params *protocol.DidCloseTextDocumentParams) (err error) {
	if err := s.openFiles.DidClose(params); err != nil {
		return err
	}

	return s.clearDiagnostics(ctx, params.TextDocument)
}
func (s *server) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) (err error) {
	if err := s.openFiles.DidChange(params); err != nil {
		return err
	}
	return s.updateDiagnostics(ctx, params.TextDocument)
}

func (s *server) DidSave(ctx context.Context, params *protocol.DidSaveTextDocumentParams) (err error) {
	s.logf("DidSave\n")

	config, err := s.loadConfig(ctx)
	if err != nil {
		return err
	}

	if config.generateOnSave {
		path, err := uriFilePath(params.TextDocument.URI)
		if err != nil {
			return err
		}

		err = s.runGenerate(ctx, filepath.Dir(path))
		if err != nil {
			_ = s.client.ShowMessage(ctx, &protocol.ShowMessageParams{
				Message: fmt.Sprintf("Failed to run `ns generate` (%v). See Namespace output for details.", err),
				Type:    protocol.MessageTypeError,
			})
		}
		return nil
	}
	return nil
}

// Formatting

func (s *server) Formatting(ctx context.Context, params *protocol.DocumentFormattingParams) (result []protocol.TextEdit, err error) {
	absPathToFormat, err := uriFilePath(params.TextDocument.URI)
	if err != nil {
		return nil, err
	}

	ws, relPath, err := s.WorkspaceForFile(ctx, absPathToFormat)
	if err != nil {
		return nil, err
	}

	// This is a little more complicated that needed: fncue assumes that it has full access to the filesystem
	// during formatting. So we provide it a virtual filesystem with editor buffers overlaid. Then we extract
	// the content it wrote out for the formatted document.
	efs := WriteOverlay(ws.FS())

	// TODO: Test that formatting new files (not present in the underlying FS) works.
	if err := fncue.Format(ctx, efs,
		fnfs.Location{RelPath: filepath.Dir(relPath)}, filepath.Base(relPath),
		fnfs.WriteFileExtendedOpts{}); err != nil {
		return nil, err
	}

	var formattedContent string
	if newContent, ok := efs.writes[relPath]; !ok {
		// If the file is already formatted correctly fncue.Format doesn't
		// prodece the write. Return immediate success.
		return nil, nil
	} else {
		formattedContent = string(newContent)
	}

	return []protocol.TextEdit{
		{
			Range:   infiniteRange,
			NewText: formattedContent,
		},
	}, nil
}

// Commands
func (s *server) ExecuteCommand(ctx context.Context, params *protocol.ExecuteCommandParams) (result interface{}, err error) {
	if params.Command == generateCommandID {
		path, ok := params.Arguments[0].(string)
		if !ok {
			return nil, jsonrpc2.NewError(jsonrpc2.InvalidRequest, "ns_generate argument must be string path")
		}
		return nil, s.runGenerate(ctx, path)
	}
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}

// Returns error only in case of an unexpected error.
// ns generate returning non-zero status is considered a success (it shows a warning internally.)
func (s *server) runGenerate(ctx context.Context, path string) error {
	_ = s.client.LogMessage(ctx, &protocol.LogMessageParams{Message: ""})
	_ = s.client.LogMessage(ctx, &protocol.LogMessageParams{
		Type:    protocol.MessageTypeInfo,
		Message: "Running `ns generate`...",
	})

	fnPath, err := os.Executable()
	if err != nil {
		return jsonrpc2.NewError(jsonrpc2.InternalError, fmt.Sprintf("failed to determine the path to the ns tool: %v", err))
	}

	cmd := exec.CommandContext(ctx, fnPath, "generate")
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	statusCode := 0

	if exitErr, isExit := err.(*exec.ExitError); isExit {
		statusCode = exitErr.ExitCode()
		err = nil
	}
	if err != nil {
		return jsonrpc2.NewError(jsonrpc2.InternalError, fmt.Sprintf("ns generate failed: %v", err))
	}

	_ = s.client.LogMessage(ctx, &protocol.LogMessageParams{Message: string(output)})

	severity := protocol.MessageTypeInfo
	if statusCode > 0 {
		// When we print an error the OutputChannel will be revealed.
		severity = protocol.MessageTypeError
	}
	_ = s.client.LogMessage(ctx, &protocol.LogMessageParams{
		Type:    severity,
		Message: fmt.Sprintf("`ns generate` finished with error code %d.\n", statusCode),
	})

	// This message is shown automatically by the languageclient with the revealOutputChannelOn=error.
	// if statusCode > 0 {
	// 	_ = s.client.ShowMessage(ctx, &protocol.ShowMessageParams{
	// 		Message: fmt.Sprintf("Failed to run `ns generate` (status code: %d). See ns output for details.", statusCode),
	// 		Type:    protocol.MessageTypeError,
	// 	})
	// }
	return err
}

func (s *server) CodeAction(ctx context.Context, params *protocol.CodeActionParams) (result []protocol.CodeAction, err error) {
	s.logf("CodeAction %v\n", params.TextDocument.URI)
	// This surfaces the generate command in the context menus.
	path, err := uriFilePath(params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	// TODO: Filter only Cue files.
	return []protocol.CodeAction{
		{
			Title:    "Run `ns generate` in the workspace",
			Kind:     "source.fixAll",
			Disabled: nil, // Why
			Command: &protocol.Command{
				Command:   generateCommandID,
				Arguments: []interface{}{path},
			},
		},
	}, nil
}

// Diagnostics

func (s *server) updateDiagnostics(ctx context.Context, document protocol.VersionedTextDocumentIdentifier) error {
	absPath, err := uriFilePath(document.URI)
	if err != nil {
		return err
	}
	ws, relPath, err := s.WorkspaceForFile(ctx, absPath)
	if err != nil {
		return err
	}

	importPath := ws.PkgNameInMainModule(relPath)
	pkgName := ws.PkgNameInMainModule(path.Dir(relPath))
	val, err := ws.EvalPackage(ctx, pkgName)
	if val.Err() != nil {
		// Prefer the errors inside the value.
		// But note that parsing errors are in the [err].
		err = val.Err()
	}

	diags := []protocol.Diagnostic{}
	errs := errors.Errors(err)
	for _, err := range errs {
		buf := strings.Builder{}
		for _, pos := range err.InputPositions() {
			fmt.Fprintf(&buf, "(%s %d %d) ", pos.Filename(), pos.Line(), pos.Column())
		}
		s.logf("updateDiagnostics root=%v err=%v ipos=%s\n", path.Dir(absPath), err, buf.String())
		var startPosition protocol.Position
		if err.Position().Line() > 0 && err.Position().Column() > 0 {
			// If line and column == 0 the error is about the file as a whole.
			startPosition = protocol.Position{
				Line:      uint32(err.Position().Line() - 1),
				Character: uint32(err.Position().Column() - 1),
			}
		} else if len(err.InputPositions()) > 0 {
			// Try to use a "related" position as an anchor.
			for _, pos := range err.InputPositions() {
				if pos.Filename() == importPath {
					startPosition = protocol.Position{
						Line:      uint32(pos.Line() - 1),
						Character: uint32(pos.Column() - 1),
					}
					break
				}
			}
		}
		// TODO: Publish InputPositions as RelatedInformation.
		diags = append(diags, protocol.Diagnostic{
			// Range is the range at which the message applies.
			Range: protocol.Range{
				Start: startPosition,
				End:   startPosition,
			},
			Severity: protocol.DiagnosticSeverityError,
			Source:   "Cue parser",
			Message:  err.Error(),
		})
	}
	return s.client.PublishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{
		URI:         document.URI,
		Version:     uint32(document.Version),
		Diagnostics: diags,
	})
}

func (s *server) clearDiagnostics(ctx context.Context, documentRef protocol.TextDocumentIdentifier) error {
	return s.client.PublishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{
		URI:         documentRef.URI,
		Diagnostics: []protocol.Diagnostic{},
	})
}

// Go to definition

func (s *server) Definition(ctx context.Context, params *protocol.DefinitionParams) (result []protocol.Location, err error) {
	absPath, err := uriFilePath(params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	s.logf("Definition %s:%d:%d\n", absPath, params.Position.Line, params.Position.Character)

	ws, wsPath, err := s.WorkspaceForFile(ctx, absPath)
	if err != nil {
		return nil, err
	}

	// First try to see if the user is pointing at an import.
	// We need to convert the position for parse stage separately since we don't yet know importPath.
	parsePos := lspPosToCue(absPath, params.Position)
	snapshot, err := s.openFiles.Read(uri.File(absPath))
	if err != nil {
		return nil, err
	}
	parsed, err := parser.ParseFile(absPath, snapshot.Text, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	matchingImport := cueImportAtPosition(parsed, parsePos)
	s.logf("matching import %v\n", matchingImport)
	if matchingImport != nil {
		return s.packageLocations(ctx, ws, matchingImport)
	}

	// Then interpret Cue values.
	fqName := ws.PkgNameInMainModule(wsPath)
	val, err := ws.EvalPackage(ctx, path.Dir(fqName))
	if err != nil {
		return nil, err
	}

	s.logf("updateDiagnostics pkg=%v err=%v val=%s\n", fqName, err, describeValue(val, ""))

	cuePos := lspPosToCue(fqName, params.Position)
	bestMatch := cueValueAtPosition(val, cuePos)
	if bestMatch == nil {
		return nil, nil
	}

	deref := cue.Dereference(*bestMatch)
	if deref == *bestMatch || deref.Source() == nil {
		return nil, nil
	}

	targetNode := deref.Source()
	targetAbsPath, err := ws.AbsPathForPkgName(ctx, targetNode.Pos().Filename())
	if err != nil {
		return nil, err
	}
	return []protocol.Location{
		{
			URI: uri.File(targetAbsPath),
			Range: protocol.Range{
				Start: cuePosToLSP(targetNode.Pos()),
				End:   cuePosToLSP(targetNode.End()),
			},
		},
	}, nil
}

func (s *server) Hover(ctx context.Context, params *protocol.HoverParams) (result *protocol.Hover, err error) {
	absPath, err := uriFilePath(params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	ws, relPath, err := s.WorkspaceForFile(ctx, absPath)
	if err != nil {
		return nil, err
	}

	importPath := ws.PkgNameInMainModule(relPath)
	pkgName := ws.PkgNameInMainModule(path.Dir(relPath))
	val, err := ws.EvalPackage(ctx, pkgName)
	if err != nil {
		// This may happen due to bad imports. Avoid freaking out over it on hover.
		return nil, nil
	}

	cuePos := lspPosToCue(importPath, params.Position)
	bestMatch := cueValueAtPosition(val, cuePos)
	s.logf("Hover %v\n", bestMatch)

	if bestMatch == nil {
		return nil, nil
	}

	var nodeRange *protocol.Range
	astNode := bestMatch.Source()
	if astNode != nil {
		nodeRange = &protocol.Range{
			Start: cuePosToLSP(astNode.Pos()),
			End:   cuePosToLSP(astNode.End()),
		}
	}

	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: fmt.Sprintf("```\n%v\n```", bestMatch),
		},
		Range: nodeRange,
	}, nil
}

// Utilities

var infiniteRange = protocol.Range{
	Start: protocol.Position{Line: 0, Character: 0},
	End:   protocol.Position{Line: math.MaxUint32, Character: math.MaxUint32},
}

// One-based UTF-8 byte offsets to zero-based UTF-16 code units.
// TODO: Handle code point conversion.
func cuePosToLSP(cuePos token.Pos) protocol.Position {
	if cuePos.Line() == 0 || cuePos.Column() == 0 {
		return protocol.Position{}
	}
	return protocol.Position{
		Line:      uint32(cuePos.Line() - 1),
		Character: uint32(cuePos.Column() - 1),
	}
}

// Zero-based UTF-16 code unit offsets to one-based UTF-8 bytes.
// TODO: Handle code point conversion.
func lspPosToCue(filename string, pos protocol.Position) token.Position {
	return token.Position{
		Filename: filename,
		Line:     int(pos.Line + 1),
		Column:   int(pos.Character + 1),
	}
}

func posInRange(pos, start, end token.Position) bool {
	if pos.Filename != start.Filename {
		return false
	}
	if start.Line > pos.Line {
		return false
	}
	if start.Line == pos.Line && start.Column > pos.Column {
		return false
	}
	if end.Line < pos.Line {
		return false
	}
	if end.Line == pos.Line && end.Column < pos.Column {
		return false
	}
	return true
}

func cueValueAtPosition(val cue.Value, pos token.Position) *cue.Value {
	var best *cue.Value
	walkCueExpressions(val, func(v cue.Value) {
		astNode := v.Source()
		if astNode == nil {
			return
		}
		if posInRange(pos, astNode.Pos().Position(), astNode.End().Position()) {
			best = &v
		}
	})
	return best
}

func cueImportAtPosition(parsed *ast.File, pos token.Position) (matchingImport *ast.ImportSpec) {
	for _, imp := range parsed.Imports {
		startPos := imp.Path.ValuePos.Position()
		endPos := imp.EndPos.Position()
		if !endPos.IsValid() {
			// EndPos is not always set, so we consider that the import ends at the EOL.
			endPos = startPos
			endPos.Line += 1
			endPos.Column = 0
		}
		if posInRange(pos, startPos, endPos) {
			matchingImport = imp
			break
		}
	}
	return
}

func (s *server) packageLocations(ctx context.Context, ws *FnWorkspace, importSpec *ast.ImportSpec) ([]protocol.Location, error) {
	importInfo, err := astutil.ParseImportSpec(importSpec)
	if err != nil {
		return nil, err
	}
	s.logf("parsed %v\n", importInfo)
	if fncue.IsStandardImportPath(importInfo.Dir) {
		// Built-in package import
		return nil, nil
	}
	absPkgPath, err := ws.AbsPathForPkgName(ctx, importInfo.Dir)
	if err != nil {
		return nil, err
	}
	// TODO: Use workspace FS for current module.
	entries, err := os.ReadDir(absPkgPath)
	if err != nil {
		return nil, err
	}
	locations := []protocol.Location{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".cue") {
			continue
		}
		absFilePath := path.Join(absPkgPath, e.Name())

		contents, err := os.ReadFile(absFilePath)
		if err != nil {
			return nil, err
		}
		parsed, err := parser.ParseFile(absFilePath, contents, parser.ImportsOnly)
		if err != nil {
			return nil, err
		}
		if parsed.PackageName() != importInfo.PkgName {
			// May be multiple packages in the directory with different names
			// referred to by "path/to/package:packageName" import spec.
			continue
		}
		locations = append(locations, protocol.Location{
			URI:   uri.File(absFilePath),
			Range: infiniteRange,
		})
	}
	return locations, nil
}

// Walk through all accessible Cue expressions from a root.
// Note that [val.Walk] only descends into list items and struct fields.
// We also descend into expression operands.
func walkCueExpressions(val cue.Value, callback func(cue.Value)) {
	callback(val)

	op, refs := val.Expr()
	if op == cue.NoOp {
		// Descend into a struct or list.
		val.Walk(func(field cue.Value) bool {
			if field != val {
				walkCueExpressions(field, callback)
			}
			return true // descend further
		}, nil)
	}
	for _, ref := range refs {
		if ref != val {
			walkCueExpressions(ref, callback)
		}
	}
}

func describeValue(v cue.Value, d string) string {
	op, refs := v.Expr()

	refS := ""
	if len(d) < 6 {
		deref := cue.Dereference(v)
		if v != deref {
			refS = refS + "\n" + describeValue(deref, d+" >")
		}

		for _, ref := range refs {
			if ref == v {
				continue
			}
			refS = refS + "\n" + describeValue(ref, d+"  ")
		}
	}

	astNode := v.Source()
	posS := "-"
	if astNode != nil {
		posS = fmt.Sprintf("%s:%d:%d-%d:%d", astNode.Pos().Filename(), astNode.Pos().Line(), astNode.Pos().Column(),
			astNode.End().Line(), astNode.End().Column())
	}

	return fmt.Sprintf("%s[%v] (%s) p=%v %s", d, op, posS, v.Path(), refS)
}

type serverConfig struct {
	generateOnSave bool
	traceServer    string
}

func (s *server) loadConfig(ctx context.Context) (serverConfig, error) {
	configResponse, err := s.client.Configuration(ctx, &protocol.ConfigurationParams{
		Items: []protocol.ConfigurationItem{{Section: "ns"}},
	})
	if err != nil {
		return serverConfig{}, fmt.Errorf(
			"couldn't access configuration for the namespace-vscode extension from the language server: %w", err)
	}
	if len(configResponse) != 1 {
		return serverConfig{}, fmt.Errorf("unexpected Configuration response: %v", configResponse)
	}
	config := configResponse[0].(map[string]interface{})
	generateOnSave, _ := config["generateOnSave"].(bool)
	traceServer, _ := config["trace.server"].(string)
	return serverConfig{
		generateOnSave: generateOnSave,
		traceServer:    traceServer,
	}, nil
}

type nonLocalURIError string

func (e nonLocalURIError) Error() string {
	return fmt.Sprintf("only local file:// URIs are supported, got %q", string(e))
}

func uriFilePath(uri protocol.URI) (string, error) {
	u, err := url.ParseRequestURI(string(uri))
	if err != nil {
		return "", err
	}
	if u.Scheme != "file" {
		return "", nonLocalURIError(string(uri))
	}
	return u.Path, nil
}

// Unimplemented

func (s *server) Declaration(ctx context.Context, params *protocol.DeclarationParams) (result []protocol.Location, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) TypeDefinition(ctx context.Context, params *protocol.TypeDefinitionParams) (result []protocol.Location, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) Implementation(ctx context.Context, params *protocol.ImplementationParams) (result []protocol.Location, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}

func (s *server) WorkDoneProgressCancel(ctx context.Context, params *protocol.WorkDoneProgressCancelParams) (err error) {
	return jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) LogTrace(ctx context.Context, params *protocol.LogTraceParams) (err error) {
	return jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) SetTrace(ctx context.Context, params *protocol.SetTraceParams) (err error) {
	return jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) CodeLens(ctx context.Context, params *protocol.CodeLensParams) (result []protocol.CodeLens, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) CodeLensResolve(ctx context.Context, params *protocol.CodeLens) (result *protocol.CodeLens, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) ColorPresentation(ctx context.Context, params *protocol.ColorPresentationParams) (result []protocol.ColorPresentation, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) Completion(ctx context.Context, params *protocol.CompletionParams) (result *protocol.CompletionList, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) CompletionResolve(ctx context.Context, params *protocol.CompletionItem) (result *protocol.CompletionItem, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) DidChangeConfiguration(ctx context.Context, params *protocol.DidChangeConfigurationParams) (err error) {
	return jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) DidChangeWatchedFiles(ctx context.Context, params *protocol.DidChangeWatchedFilesParams) (err error) {
	return jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) DidChangeWorkspaceFolders(ctx context.Context, params *protocol.DidChangeWorkspaceFoldersParams) (err error) {
	return jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) DocumentColor(ctx context.Context, params *protocol.DocumentColorParams) (result []protocol.ColorInformation, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) DocumentHighlight(ctx context.Context, params *protocol.DocumentHighlightParams) (result []protocol.DocumentHighlight, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) DocumentLink(ctx context.Context, params *protocol.DocumentLinkParams) (result []protocol.DocumentLink, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) DocumentLinkResolve(ctx context.Context, params *protocol.DocumentLink) (result *protocol.DocumentLink, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) DocumentSymbol(ctx context.Context, params *protocol.DocumentSymbolParams) (result []interface{}, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) FoldingRanges(ctx context.Context, params *protocol.FoldingRangeParams) (result []protocol.FoldingRange, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) OnTypeFormatting(ctx context.Context, params *protocol.DocumentOnTypeFormattingParams) (result []protocol.TextEdit, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) PrepareRename(ctx context.Context, params *protocol.PrepareRenameParams) (result *protocol.Range, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) RangeFormatting(ctx context.Context, params *protocol.DocumentRangeFormattingParams) (result []protocol.TextEdit, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) References(ctx context.Context, params *protocol.ReferenceParams) (result []protocol.Location, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) Rename(ctx context.Context, params *protocol.RenameParams) (result *protocol.WorkspaceEdit, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) SignatureHelp(ctx context.Context, params *protocol.SignatureHelpParams) (result *protocol.SignatureHelp, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) Symbols(ctx context.Context, params *protocol.WorkspaceSymbolParams) (result []protocol.SymbolInformation, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) WillSave(ctx context.Context, params *protocol.WillSaveTextDocumentParams) (err error) {
	return jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) WillSaveWaitUntil(ctx context.Context, params *protocol.WillSaveTextDocumentParams) (result []protocol.TextEdit, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) ShowDocument(ctx context.Context, params *protocol.ShowDocumentParams) (result *protocol.ShowDocumentResult, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) WillCreateFiles(ctx context.Context, params *protocol.CreateFilesParams) (result *protocol.WorkspaceEdit, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) DidCreateFiles(ctx context.Context, params *protocol.CreateFilesParams) (err error) {
	return jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) WillRenameFiles(ctx context.Context, params *protocol.RenameFilesParams) (result *protocol.WorkspaceEdit, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) DidRenameFiles(ctx context.Context, params *protocol.RenameFilesParams) (err error) {
	return jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) WillDeleteFiles(ctx context.Context, params *protocol.DeleteFilesParams) (result *protocol.WorkspaceEdit, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) DidDeleteFiles(ctx context.Context, params *protocol.DeleteFilesParams) (err error) {
	return jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) CodeLensRefresh(ctx context.Context) (err error) {
	return jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) PrepareCallHierarchy(ctx context.Context, params *protocol.CallHierarchyPrepareParams) (result []protocol.CallHierarchyItem, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) IncomingCalls(ctx context.Context, params *protocol.CallHierarchyIncomingCallsParams) (result []protocol.CallHierarchyIncomingCall, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) OutgoingCalls(ctx context.Context, params *protocol.CallHierarchyOutgoingCallsParams) (result []protocol.CallHierarchyOutgoingCall, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) SemanticTokensFull(ctx context.Context, params *protocol.SemanticTokensParams) (result *protocol.SemanticTokens, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) SemanticTokensFullDelta(ctx context.Context, params *protocol.SemanticTokensDeltaParams) (result interface{}, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) SemanticTokensRange(ctx context.Context, params *protocol.SemanticTokensRangeParams) (result *protocol.SemanticTokens, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) SemanticTokensRefresh(ctx context.Context) (err error) {
	return jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) LinkedEditingRange(ctx context.Context, params *protocol.LinkedEditingRangeParams) (result *protocol.LinkedEditingRanges, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) Moniker(ctx context.Context, params *protocol.MonikerParams) (result []protocol.Moniker, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
func (s *server) Request(ctx context.Context, method string, params interface{}) (result interface{}, err error) {
	return nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "unimplemented")
}
