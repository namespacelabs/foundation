# Publishing

## Setup

Install `gcloud` and `npx`.

## Publish

```bash
# From time to time.
npx google-artifactregistry-auth

# It will ask to bump the version.
# "--no-git-tag-version" is important, otherwise it will create a git tag with the new version and will automatically commit the package.json changes.
yarn publish --no-git-tag-version
```

## Notes

The registry address needs to be in two places: `.npmrc` and `.yarnrc`.

`.npmrc` is needed for `npx google-artifactregistry-auth`, and Yarn uses it to determine the
registry.

But without `.yarnrc` Yarn would specify the wrong tarball URL in the published package.
