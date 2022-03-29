// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// XXX This file is generated.

import yargs from "yargs/yargs";
import { Server, ServerCredentials } from "@grpc/grpc-js";

const argv = yargs(process.argv.slice(2))
  .options({
    listen_hostname: { type: "string" },
    port: { type: "number" },
  })
  .parse();

const server = new Server();

console.log(`Starting to listen on ${argv.listen_hostname}:${argv.port}`);

server.bindAsync(
  `${argv.listen_hostname}:${argv.port}`,
  ServerCredentials.createInsecure(),
  () => {
    server.start();
  }
);