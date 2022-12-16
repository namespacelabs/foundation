# Contributing to Namespace

## Developing

Developing `ns` requires Docker and `nix`. Use your operating system's preferred method to install
Docker (e.g. Docker Desktop on MacOS).

We use `nix` to guarantee a stable development environment (it's not used throughout yet, e.g. for
releases, but that's a work in progress).

- [Install `nix`](https://nixos.org/download.html) to your target environment. Both Linux (and WSL2)
  as well as MacOS are supported.

Namespace is regularly tested under Linux, MacOS 11+ and Windows WSL2.

After `nix` is installed, you can:

- Use `nix-shell` to jump into a shell with all the current dependencies setup (e.g. Go, NodeJS,
  etc).
- Use the "nix environment selector" VSCode extension to apply a nix environment in VSCode.
- Or use the pre-configured VSCode devcontainer.

### Backwards-incompatible changes

If you're making a backwards-incompatible change (e.g. removing support for an existing syntax),
please make sure to update the version number in `internal/versions/version.go` and
`ns-workspace.cue`. This will instruct `ns` to tell the user that it needs to be updated.

## Building

```bash
git clone git@github.com:namespacelabs/foundation.git
go install -v ./cmd/ns
ns
```

## Committing

We use `pre-commit` to enforce consistent code formatting. Please,
[install `pre-commit`](https://pre-commit.com/#install).

[By default](https://pre-commit.com/#usage), `pre-commit` requires you to run `pre-commit install`
for _every_ cloned repo containing a `.pre-commit-config.yaml` file.

Alternatively, you can
[configure your local git](https://pre-commit.com/#automatically-enabling-pre-commit-on-repositories)
to run `pre-commit` for each relevant repository automatically.

```bash
git config --global init.templateDir ~/.git-template
pre-commit init-templatedir ~/.git-template
```

## Development Workflows

### nsdev

`nsdev` is a copy of `ns` that also embeds additional debug/maintenance features.
Install it with `go install ./cmd/nsdev` to unlock all debug workflows below.

### Debugging via VS Code

The debugging configuration is not in the repository because different people may want to used
different arguments. To bootstrap, create `.vscode/launch.json` and add the following content:

```bash
{
  // Use IntelliSense to learn about possible attributes.
  // Hover to view descriptions of existing attributes.
  // For more information, visit: https://go.microsoft.com/fwlink/?linkid=830387
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Debug Foundation",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceRoot}/cmd/ns",
      // This is important, otherwise debugging doesn't work.
      "env": { "CGO_ENABLED": "0" },
      // Specify the absolute path to the working directory.
      "cwd": "~/code/foundation",
      "args": ["generate"]
    }
  ]
}
```

### Making changes to `internal/runtime` and orchestrator code

Part of `ns` is deployed into the target cluster, to orchestrate changes on behalf of the user.
Historically, all orchestration was done by `ns`, so the orchestration code is still built into
`ns`.

As you're doing changes to `internal/runtime` or any "execution op" that is used as part of
deployment, you may find that your changes are not reflected as part of your debugging session.

This is because:

(a) That code by default runs inside of the cluster, not within `ns`.

(b) By default `ns` install a prebuilt orchestrator, rather than deploying one with your changes.

You can modify this behavior using one of:

- `--use_orchestrator=false` asks `ns` to not use the in-cluster orchestrator, but defer instead to
  the orchestration that is built into itself (this will be eventually removed).

- `--use_pinned_orchestrator=false` asks `ns` to build an orchestrator from foundation's codebase,
  rather than deploying one from a prebuilt. This guarantees that it includes any changes you're
  making.

### Protos

We use protos in various parts of our codebase. Code gen for protos is managed manually using [`nsdev`](#nsdev).
You can run `nsdev source protogen` to update the generated files. E.g.

```bash
nsdev source protogen schema/
```

At times you'll also see JSON being used directly, this is often of two forms:

(a) We use protojson to ship proto values to the frontend, as there's no great jspb (or equivalent)
support for us to rely on.

(b) Adding one more proto was cumbersome, and for iteration purposes we ended up with inline JSON
definitions. It's a hard iteration speed trade-off (because of the manual codegen bit), but we
should lean on protos more permanently.

### Inspect computed schemas

Any node type:

```bash
nsdev debug print-computed internal/testdata/server/gogrpc
```

For servers:

```bash
nsdev debug print-sealed internal/testdata/server/gogrpc
```

### Debugging latency

`ns` exports tracing information if configured to use Jaeger.

First, Jaeger needs to be running.

```bash
nsdev debug prepare
```

And then configure `ns` to push traces, either set `enable_tracing` unconditionally in
`~/.config/fn/config.json` or per invocation:

```bash
FN_ENABLE_TRACING=true ns build ...
```

Check out the trace at [http://localhost:20000/](http://localhost:20000/).

### Iterating on the internal Dev UI

```bash
ns dev --devweb internal/testdata/server/gogrpc
```

Adding `--devweb` starts a development web frontend. Yarn and NodeJS are required for `--devweb`.
Also, run `yarn install` in the `devworkflow/web` directory to fetch and link node dependencies.

```bash
ns dev -H 0.0.0.0:4001 --devweb internal/testdata/server/gogrpc
```

Use `-H` to change the listening hostname/port, in case you're running `ns dev` in a machine or VM
different from your workstation.

### Using `age` for simple secret management

When a server has secrets required for deployment, sharing those secrets between different users can
sometimes be challenging. Namespace includes a simple solution for it, building on `age`.

Users generate pub/private identities using `ns keys generate`, which can then be used to encrypt
"secret bundles" which are submittable into the repository. Access to the payload is determined by
the keys which have been added as receipients to the encrypted payload. This list of keys is public,
and kept in the repository as part of the bundle.

```bash
$ ns keys generate
Created age1kacjakcg8dqyxzdwldemrx4pt79ructa6z0mgw7nk03mgxl3vqsslph4fz
```

```bash
$ ns secrets set internal/testdata/server/gogrpc --secret namespacelabs.dev/foundation/internal/testdata/datastore:cert

Specify a value for "cert" in namespacelabs.dev/foundation/internal/testdata/datastore.

Value: <value>

Wrote internal/testdata/server/gogrpc/server.secrets
```

A `server.secrets` will be produced which can be submitted to the repository, as the secret values
are encrypted.

To grant access to the encrypted file, merely have your teammate generate a key (see above), add
run:

```bash
$ ns secrets add-reader internal/testdata/server/gogrpc --key <pubkey>
Wrote internal/testdata/server/gogrpc/server.secrets
```

The resulting file can then be submitted to the repository.

To inspect who has access to the bundle, and which secrets are stored, run:

```bash
$ ns secrets info internal/testdata/server/gogrpc
Readers:
  age1mlefr5zhnesgzfl7aefy95qlem0feuyfpdpmee6lk50x4h6mlskqdffjxv
Definitions:
  namespacelabs.dev/foundation/internal/testdata/datastore:cert
```

Note: this mechanism for secret management does not handle revocations. If a key has been issued
which should no longer have access to the contents, all secret values should be considered
compromised and replaced (as the person with private key can read the values from any previous
repository commit).
