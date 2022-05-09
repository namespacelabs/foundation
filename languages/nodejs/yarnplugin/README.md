# Foundation plugin for Yarn

This plugin allows to resolve `fn:` paths in dependencies, leveraging the Foundation cache to fetch
the files.

## Implementation

This is a fork of the
[Yarn "file" plugin](https://github.com/yarnpkg/berry/tree/%40yarnpkg/cli/3.2.0/packages/plugin-file),
with the following changes:

- Using `fn:` protocol name instead of `file:`.
- File paths are resolved using the Foundation cache.

## Distribution

The plugin content is embedded into the `fn` binary. `fn tidy` writes the plugin content to all Yarn
roots under `.yarn/plugins` and this code is submitted to the user repository, as recommended by
Yarn.

In the future we may reconsider this behavior, for example, add the plugin to `.gitignore`.

### Why not an NPM package?

From the Yarn help:

```
Plugins cannot be downloaded from the npm registry, and aren't allowed to have
dependencies (they need to be bundled into a single file, possibly thanks to the
`@yarnpkg/builder` package).
```
