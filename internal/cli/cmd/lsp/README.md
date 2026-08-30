# Ns LSP

`ns lsp` command implements language server protocol. It uses stdin/stdout as JSON-RPC2 medium.

The LSP can be used by various editors and IDEs. Notably `fn-vscode` extension plumbs this LSP
into VSCode.

The LSP is implemented inside the `ns` binary to make sure that it editor integrations behave 100%
consistent with the command line tools.

## References

* Upstream tracking issue https://github.com/cue-lang/cue/issues/142
* LSP Specification https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/
* Cue Language specification https://cuelang.org/docs/references/spec/

## Related work

* Cue LSP draft implementation https://github.com/galli-leo/cue-lsp-server/
* Go LSP framework https://github.com/go-language-server/jsonrpc2
* Go LSP type definitions https://github.com/sourcegraph/go-lsp/
* Another Go LSP framework https://github.com/TobiasYin/go-lsp/
* Private JSONRPC implementation https://pkg.go.dev/golang.org/x/tools/internal/jsonrpc2

## Examples

* VSCode CUE syntax https://github.com/ngkcl/vscode-cuelang
* Abandoned 3p Golang LSP https://github.com/sourcegraph/go-langserver/blob/master/langserver/handler.go
* VSCode->Gopls integration https://github.com/golang/vscode-go/blob/master/src/language/goLanguageServer.ts
* VSCode LSP sample https://github.com/microsoft/vscode-extension-samples/tree/main/lsp-sample/
