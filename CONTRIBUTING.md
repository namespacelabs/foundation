# Contributing to Foundation

This guide is targeted at the Namespace Labs team for now.

## Developing

Developing `ns` requires Docker and `nix`. Use your operating system's preferred method to install
Docker (e.g. Docker Desktop on MacOS).

We use `nix` to guarantee a stable development environment (it's not used throughout yet, e.g. for
releases, but that's a work in progress).

- [Install `nix`](https://nixos.org/download.html) to your target environment. Both Linux (and WSL2)
  as well as MacOS are supported.

Foundation is regularly tested under Linux, MacOS 11+ and Windows WSL2.

After `nix` is installed, you can:

- Use `nix-shell` to jump into a shell with all the current dependencies setup (e.g. Go, NodeJS,
  etc).
- Use the "nix environment selector" VSCode extension to apply a nix environment in VSCode.
- Or use the pre-configured VSCode devcontainer.

## Building

```bash
git clone git@github.com:namespacelabs/foundation.git
go install -v ./cmd/ns
ns
```

## Committing

We use `pre-commit` to enforce consistent code formatting.
Please, [install `pre-commit`](https://pre-commit.com/#install).

[By default](https://pre-commit.com/#usage), `pre-commit` requires you to run `pre-commit install` for _every_ cloned repo containing a `.pre-commit-config.yaml` file.

Alternatively, you can [configure your local git](https://pre-commit.com/#automatically-enabling-pre-commit-on-repositories) to run `pre-commit` for each relevant repository automatically.

```bash
git config --global init.templateDir ~/.git-template
pre-commit init-templatedir ~/.git-template
```

## Releasing

We use `goreleaser` for our releases. You should have it under your `nix-shell`.

Our releases are published to:

- [GitHub releases](https://github.com/namespacelabs/foundation/releases),
- [Public S3 bucket](https://s3.console.aws.amazon.com/s3/buckets/ns-releases).
  This allows end-users easily download binaries without messing with GitHub authentication
  to access the private repos.
- [Homebew TAP](https://github.com/namespacelabs/homebrew-namespace)

We have two distict packages to release:

- `ns` core binary, defined in `.goreleaser.yaml` which contains the actual commands.
  Released to GitHub releases and to the public S3 bucket.
- `nsboot` binary, defined in `.goreleaser.nsboot.yaml` which is a thin wrapper
  which downloads and runs the appropriate version of `ns` upon every invocation.
  This is intended to be the primary entry point for end users and is published
  to the package repositories (along withj GitHub releases and the S3 bucket).

You can test a release by running:

```bash
goreleaser release --rm-dist --snapshot
```

Our versioning scheme uses a ever-increasing minor version. After `0.0.23` comes `0.0.24`, and so
on.

To issue an actual release:

1. Create a Github PAT with `write_packages` permissions and place it in
   `~/.config/goreleaser/github_token`. This allows GoReleaser to upload to Github releases.
1. Log into AWS with `aws --profile prod-main sso login`.
1. Export AWS temporary credentials with [aws-sso-creds](https://github.com/jaxxstorm/aws-sso-creds#installation)
   `aws-sso-creds set default -p prod-main`.
1. Pick a new version (check the existing tag list): `git tag -a v0.0.24`
1. Run the release `goreleaser release --rm-dist` (add `-f .goreleaser.nsboot.yaml` to release `nsboot`).
1. When releasing `nsboot` update the version in `install/install.sh`.
1. After the release is complete, remember to remove the `dist` directory to keep your workspace size small.

NOTE: all commits end up in an automatically generated changelog. Commits that include `docs:`,
`test:` or `nochangelog` are excluded from the changelog.

### MacOS Notarization

Note: currently the notarization is not required. Namespace binaries are downloaded by Homebrew and `nsboot`
and these tools do not set the quarantine flag (see [SO](https://stackoverflow.com/questions/67446317/why-are-executables-installed-with-homebrew-trusted-on-macos), verified on a fresh macOS install by Kirill).

If needed, notarization is to be done in MacOSX, and requires XCode, and
[gon](https://github.com/mitchellh/gon#installation). Currently Hugo is the only person to perform notarization
as he posesses the right Apple Developer Certificate.

## Development Workflows

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

### Protos

We use protos in various parts of our codebase. Code gen for protos is managed manually. You can run
`ns source protogen` to update the generated files. E.g.

```bash
ns source protogen schema/
```

At times you'll also see JSON being used directly, this is often of two forms:

(a) We use protojson to ship proto values to the frontend, as there's no great jspb (or equivalent)
support for us to rely on.

(b) Adding one more proto was cumbersome, and for iteration purposes we ended up with inline JSON
definitions. It's a hard iteration speed trade-off (because of the manual codegen bit), but we
should lean on protos more permanently.

### Changing node definitions (extensions, service, server)

Changing a definition which impacts codegen (i.e. services are exported, or extension
initializations are registered), requires a codegen refresh. This is done automatically as a part of
`ns build/deploy/dev` but can also be triggered manually:

```bash
ns generate
```

### Rebuild prebuilts

At the moment, "prebuilts" are stored in GCP's Artifact Registry. Accessing these packages requires
no authentication. However, you'll have to sign-in in order to update these prebuilts.

Run `gcloud auth login` to authenticate to GCP with your `namespacelabs.com` account, and then
whenever you need to write new prebuilt images, you'll have to run:

#### All prebuilts

```bash
ns build-binary --all --build_platforms=linux/arm64,linux/amd64 \
     --output_prebuilts --base_repository=us-docker.pkg.dev/foundation-344819/prebuilts/ --log_actions
```

#### Specific images

```bash
nsdev build-binary std/networking/gateway/controller std/networking/gateway/server/configure \
     --build_platforms=linux/arm64,linux/amd64 --output_prebuilts \
     --base_repository=us-docker.pkg.dev/foundation-344819/prebuilts/ --log_actions
```

You can then update the `prebuilt_binary` definitions in `workspace.textpb` with the values above.

### Inspect computed schemas

Any node type:

```bash
nsdev debug print-computed std/testdata/server/gogrpc
```

For servers:

```bash
nsdev debug print-sealed std/testdata/server/gogrpc
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
ns dev --devweb std/testdata/server/gogrpc
```

Adding `--devweb` starts a development web frontend. Yarn and NodeJS are required for `--devweb`.
Also, run `yarn install` in the `devworkflow/web` directory to fetch and link node dependencies.

```bash
ns dev -H 0.0.0.0:4001 --devweb std/testdata/server/gogrpc
```

Use `-H` to change the listening hostname/port, in case you're running `ns dev` in a machine or VM
different from your workstation.

### Using `age` for simple secret management

When a server has secrets required for deployment, sharing those secrets between different users can
sometimes be challenging. Foundation includes a simple solution for it, building on `age`.

Users generate pub/private identities using `ns keys generate`, which can then be used to encrypt
"secret bundles" which are submittable into the repository. Access to the payload is determined by
the keys which have been added as receipients to the encrypted payload. This list of keys is public,
and kept in the repository as part of the bundle.

```bash
$ ns keys generate
Created age1kacjakcg8dqyxzdwldemrx4pt79ructa6z0mgw7nk03mgxl3vqsslph4fz
```

```bash
$ ns secrets set std/testdata/server/gogrpc --secret namespacelabs.dev/foundation/std/testdata/datastore:cert

Specify a value for "cert" in namespacelabs.dev/foundation/std/testdata/datastore.

Value: <value>

Wrote std/testdata/server/gogrpc/server.secrets
```

A `server.secrets` will be produced which can be submitted to the repository, as the secret values
are encrypted.

To grant access to the encrypted file, merely have your teammate generate a key (see above), add
run:

```bash
$ ns secrets add-reader std/testdata/server/gogrpc --key <pubkey>
Wrote std/testdata/server/gogrpc/server.secrets
```

The resulting file can then be submitted to the repository.

To inspect who has access to the bundle, and which secrets are stored, run:

```bash
$ ns secrets info std/testdata/server/gogrpc
Readers:
  age1mlefr5zhnesgzfl7aefy95qlem0feuyfpdpmee6lk50x4h6mlskqdffjxv
Definitions:
  namespacelabs.dev/foundation/std/testdata/datastore:cert
```

Note: this mechanism for secret management does not handle revocations. If a key has been issued
which should no longer have access to the contents, all secret values should be considered
compromised and replaced (as the person with private key can read the values from any previous
repository commit).
