// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

"use strict";

const express = require("express");
const fs = require("fs");

const nsConfigRaw = fs.readFileSync("/namespace/config/runtime.json");
const nsConfig = JSON.parse(nsConfigRaw);
console.log(`Namespace config: ${JSON.stringify(nsConfig, null, 2)}`);

const nsResourcesRaw = fs.readFileSync("/namespace/config/resources.json");
const nsResources = JSON.parse(nsResourcesRaw);
console.log(`Namespace resources: ${JSON.stringify(nsResources, null, 2)}`);

// Constants
const PORT = nsConfig.current.port.find((s) => s.name === "webapi").port;
const HOST = "0.0.0.0";

// App
const app = express();
app.get("/readyz", (req, res) => {
	res.send("all ok");
});

app.get("/mypath", (req, res) => {
	// Accessing the env variables from cue file
	res.send(`Hello from complex docker, ${process.env.NAME}!`);
});

app.listen(PORT, HOST);

console.log(`Running on http://${HOST}:${PORT}`);
