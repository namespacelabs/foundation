// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import express from "express";
import { readFileSync } from "fs";

const nsConfigRaw = readFileSync("/namespace/config/runtime.json");
const nsConfig = JSON.parse(nsConfigRaw.toString());
console.log(`Namespace config: ${JSON.stringify(nsConfig, null, 2)}`);

// Constants
const PORT = nsConfig.current.port.find((s: any) => s.name === "webapi").port;
const HOST = "0.0.0.0";

// App
const app = express();
app.get("/", (_: any, res: express.Response<any, any>) => {
	// Accessing the env variables from cue file
	res.send(`Hello, ${process.env.NAME}!`);
});

app.listen(PORT, HOST);

console.log(`Running on http://${HOST}:${PORT}`);
