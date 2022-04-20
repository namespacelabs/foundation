
# Contributing to Foundation

This guide is targeted at the Namespace Labs team for now.

## Developing

Developing `fn` requires Docker and `nix`. Use your operating system's preferred method to install
Docker (e.g. Docker Desktop on MacOS).

We use `nix` to guarantee a stable development environment (it's not used throughout yet, e.g. for
releases, but that's a work in progress).

- Install `nix` to your target environment, https://nixos.org/download.html. Both Linux (and WSL2)
  as well as MacOS are supported.

Foundation is regularly tested under Linux, MacOS 11+ and Windows WSL2.

After `nix` is installed, you can:

- Use `nix-shell` to jump into a shell with all the current dependencies setup (e.g. Go, NodeJS, etc).
- Use the "nix environment selector" VSCode extension to apply a nix environment in VSCode.
- Or use the pre-configured VSCode devcontainer.

## Building

```bash
git clone git@github.com:namespacelabs/foundation.git
go install -v ./cmd/fn
fn
```

## Releasing

We use `goreleaser` for our releases. You should have it under your `nix-shell`.

You can test a release by running:

```bash
goreleaser release --rm-dist --snapshot
```

To issue an actual release, create a Github PAT with `write_packages` permissions and place it in `~/.github/github_token`.

Our versioning scheme uses a ever-increasing minor version. After `0.0.23` comes `0.0.24`, and so on.

Pick a new version, and run:

```bash
git tag -a v0.0.24
goreleaser release
```

NOTE: all commits end up in an automatically generated changelog. Commits that include `docs:`, `test:` or `nochangelog` are excluded from the changelog.

### MacOS Notarization

In order to allow `fn` binaries to be installed outside of the App store, they need to be notarized.

Notarization must be done in MacOSX, and requires XCode, and https://github.com/mitchellh/gon#installation.

## Development Workflows

### Protos

We use protos in various parts of our codebase. Code gen for protos is managed manually. You can run
`fn source protogen` to update the generated files.

At times you'll also see JSON being used directly, this is often of two forms:

(a) We use protojson to ship proto values to the frontend, as there's no great jspb (or equivalent)
support for us to rely on.

(b) Adding one more proto was cumbersome, and for iteration purposes we ended up with inline JSON
definitions. It's a hard iteration speed trade-off (because of the manual codegen bit), but we
should lean on protos more permanently.

### Changing node definitions (extensions, service, server)

Changing a definition which impacts codegen (i.e. services are exported, or extension
initializations are registered), requires a codegen refresh. This is done automatically as a part of
`fn build/deploy/dev` but can also be triggered manually:

```bash
fn generate
```

### Rebuild prebuilts

At the moment, "prebuilts" are stored in GCP's Artifact Registry. Accessing these packages
requires no authentication. However, you'll have to sign-in in order to update these prebuilts.

Run `gcloud auth login` to authenticate to GCP with your `namespacelabs.com` account, and then
whenever you need to write new prebuilt images, you'll have to run:

```bash
fn build-binary --all --build_platforms=linux/arm64,linux/amd64 \
     --output_prebuilts --base_repository=us-docker.pkg.dev/foundation-344819/prebuilts/ --log_actions
```

You can then update the `prebuilt_binary` definitions in `workspace.textpb` with the values above.

### Inspect computed schemas

Any node type:

```bash
fndev debug print-computed std/testdata/server/gogrpc
```

For servers:

```bash
fndev debug print-sealed std/testdata/server/gogrpc
```

### Debugging latency

`fn` exports tracing information if configured to use Jaeger.

First, Jaeger needs to be running.

```bash
fndev debug prepare
```

And then configure `fn` to push traces, either set `enable_tracing` unconditionally in
`~/.config/fn/config.json` or per invocation:

```bash
FN_ENABLE_TRACING=true fn build ...
```

Check out the trace at http://localhost:20000/.

### Updating dependencies

To keep dependencies under check, we rely on https://github.com/tailscale/depaware to produce an expanded
list of transitive dependencies, which is meant to be reviewed manually.

If package imports change, a new depaware often needs to be recreated. Use the following commmand to
re-create:

```bash
go run github.com/tailscale/depaware --update namespacelabs.dev/foundation/cmd/fn
```

### Iterating on the internal Dev UI

```bash
fn dev --devweb std/testdata/server/gogrpc
```

Adding `--devweb` starts a development web frontend. Yarn and NodeJS are required for `--devweb`.
Also, run `yarn install` in the `devworkflow/web` directory to fetch and link node dependencies.

```bash
fn dev -H 0.0.0.0:4001 --devweb std/testdata/server/gogrpc
```

Use `-H` to change the listening hostname/port, in case you're running `fn dev` in a machine or VM
different from your workstation.

### Using `age` for simple secret management

When a server has secrets required for deployment, sharing those secrets between different users
can sometimes be challenging. Foundation includes a simple solution for it, building on `age`.

You generate a pub/private identity using `fn keys generate`, which can then be
used to encrypt a payload that can be submitted back into a repository. Access
to the payload is determined by the keys which have been added as receipients to
the encrypted payload. This list of keys can be public, and kept in the
repository.

```
fn keys generate
Created age1kacjakcg8dqyxzdwldemrx4pt79ructa6z0mgw7nk03mgxl3vqsslph4fz
```


```
mkdir server/secrets/
echo age1kacjakcg8dqyxzdwldemrx4pt79ructa6z0mgw7nk03mgxl3vqsslph4fz >> server/secrets/contents.tar.keys
fn keys encrypt server/secrets
```

A `server/secrets/contents.tar.age` will be generated which can be submitted to the repository. 

To grant access to the encrypted file, merely have your teammate generate a key (see above), add it to
the keys file, and run:

```
fn keys encrypt --reencrypt server/secrets
```

The resulting files should now be submitted to the repository.

To perform other changes, simply do:

```
fn keys shell server/secrets/
```

And you'll be dropped to a shell where the secrets have been decrypted, and will be re-encrypted when you leave
the shell.

Note: this mechanism for secret management does not handle revocations. If a key has been issued which should
no longer have access to the contents, all secret values should be considered compromised and replaced (as the
person with private key can read the values from any previous repository commit).
