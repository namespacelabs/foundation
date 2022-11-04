// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

const { defineConfig } = require("cypress");

module.exports = defineConfig({
	fixturesFolder: false,
	viewportWidth: 500,
	viewportHeight: 200,
	e2e: { supportFile: false },
});
