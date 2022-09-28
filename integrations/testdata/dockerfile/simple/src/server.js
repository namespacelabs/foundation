// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

"use strict";

const express = require("express");
const fs = require("fs");

// Reading and parsing the Namespace runtime config to get the port.
const nsConfig = JSON.parse(fs.readFileSync("/namespace/config/runtime.json"));

// Constants
const PORT = nsConfig.current.port[0].port;
const HOST = "0.0.0.0";

// App
const app = express();
app.get("/", (_, res) => res.send(`Hello from simple docker, world!`));

app.listen(PORT, HOST);

console.log(`Running on http://${HOST}:${PORT}`);
