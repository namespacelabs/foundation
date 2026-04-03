---
name: releasing-nsc
description: Creates a new nsc release tag in the foundation repository and verifies the repo is using the expected lightweight v0.0.x release flow. Use when asked to cut, tag, or publish a new nsc release.
---

# Releasing NSC

Creates a new `nsc` release by tagging this repository with the next `v0.0.x` version.

This repository publishes `nsc` from the shared repo tag, not from a separate `nsc/*` tag namespace. The release config in `.goreleaser.yaml` builds the `nsc` binaries from the repo version, and `install/install_nsc.sh` downloads artifacts from `https://get.namespace.so/packages/nsc/v<version>/...`.

## Workflow

1. Fetch remote tags before choosing a version.

```bash
git fetch --tags --force
```

2. Inspect the latest released versions.

```bash
git tag --sort=version:refname | tail -20
git describe --tags --always --dirty
git log --oneline "$(git tag --sort=version:refname | tail -1)"..HEAD
```

3. Confirm the latest tag style is still a lightweight repo tag.

```bash
LATEST_TAG="$(git tag --sort=version:refname | tail -1)"
git for-each-ref "refs/tags/${LATEST_TAG}" --format='%(refname:short) %(objecttype) %(subject)'
```

If the object type is `commit`, keep using a lightweight tag with `git tag <version>`.

4. Pick the next patch version unless the user requested an explicit version.

Example: if the latest tag is `v0.0.492`, create `v0.0.493`.

5. Create the tag on `HEAD`.

```bash
git tag v0.0.493
```

6. Push only the new tag.

```bash
git push origin v0.0.493
```

7. Verify the tag exists locally and on the remote.

```bash
git show --stat --no-patch v0.0.493
git ls-remote --tags origin "v0.0.493"
```

## Checks

- Do not assume `nsc` has a separate tag namespace; verify `.goreleaser.yaml` first if anything looks unusual.
- Do not replace existing tags.
- Prefer the smallest correct release action: fetch, inspect, tag, push, verify.
- If `HEAD` is already tagged with the latest version, stop and tell the user instead of creating a duplicate release.

## Repo-Specific References

- `.goreleaser.yaml`
- `install/install_nsc.sh`
- `cmd/nsc/main.go`
