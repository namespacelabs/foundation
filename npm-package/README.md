# @namespacelabs/cli

npm wrapper for the [Namespace](https://namespace.so) CLI (`nsc`).

## Usage

Run nsc commands directly with npx:

```bash
npx @namespacelabs/cli list
npx @namespacelabs/cli build
```

Or install globally:

```bash
npm install -g @namespacelabs/cli
nsc list
```

## Update

To update to the latest version of nsc:

```bash
npx @namespacelabs/cli update
```

## How it works

On first run, the package downloads the appropriate nsc binary for your platform (macOS or Linux, amd64 or arm64) and caches it locally. Subsequent runs use the cached binary.

## License

Apache-2.0
