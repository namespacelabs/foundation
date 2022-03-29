A `nodejs` server assumes a few things that are not currently verified:

- It assumes that Yarn is being used as a package manager.
- That a `build` target exists which prepares the server for serving.
- That a `serve` target exists in `package.json` which runs the server in a blocking way, i.e. it not exit.
- That an HTTP port is listened to at port 8080.
