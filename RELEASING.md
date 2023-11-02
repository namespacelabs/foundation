# Releasing Namespace

This guide is targeted at the Namespace Labs team.

## Releasing

We use `goreleaser` for our releases. You should have it under your `nix-shell`.

Our releases are published to:

- [GitHub releases](https://github.com/namespacelabs/foundation/releases),
- [Homebrew TAP](https://github.com/namespacelabs/homebrew-namespace)

Before releasing a new `ns` version, please rebuild `ns` and verify that `ns test --all` passes in other
repos (e.g. examples, internal).

You can test a release by running:

```bash
goreleaser release --clean --snapshot
```

Our versioning scheme uses a ever-increasing minor version. After `0.0.23` comes `0.0.24`, and so
on.

To issue an actual release:

1. Create a Github PAT with `write_packages` permissions and place it in
   `~/.config/goreleaser/github_token`. This allows GoReleaser to upload to Github releases.
2. Pick a new version (check the existing tag list): `git tag v0.0.24`
3. Run the release `goreleaser release --clean`.
4. After the release is complete, remember to remove the `dist` directory to keep your workspace
   size small.
5. **Important**: this creates a _draft_ release in GitHub. Remember to publish it manually from GitHub web UI
once every artifact is fully uploaded.

NOTE: all commits end up in an automatically generated changelog. Commits that include `docs:`,
`test:` or `nochangelog` are excluded from the changelog.

NOTE: if you are running `goreleaser` inside a nix shell on macOS, you'll need to specify your host compiler, e.g.:

```bash
CC=/usr/bin/gcc goreleaser release --clean --snapshot
```

### MacOS Notarization

Note: currently the notarization is not required. Namespace binaries are downloaded by Homebrew and
`nsboot` and these tools do not set the quarantine flag (see
[SO](https://stackoverflow.com/questions/67446317/why-are-executables-installed-with-homebrew-trusted-on-macos),
verified on a fresh macOS install by Kirill).

If needed, notarization is to be done in MacOSX, and requires XCode, and
[gon](https://github.com/mitchellh/gon#installation). Currently Hugo is the only person to perform
notarization as he posesses the right Apple Developer Certificate.

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

### Changing node definitions (extensions, service, server)

Some servers that you might touch still use our application framework (sometimes referred to as old syntax).
Changing their definition can impact codegen (i.e. services are exported, or extension
initializations are registered), requires a codegen refresh. This is done automatically as a part of
`ns build/deploy/dev` but can also be triggered manually:

```bash
ns generate
```
