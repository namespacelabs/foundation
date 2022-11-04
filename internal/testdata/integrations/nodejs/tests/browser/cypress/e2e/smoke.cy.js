// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

describe("My First Test", () => {
	it("Smoke", () => {
		// Using the Cypress API to read the Namespace runtime config.
		cy.readFile("/namespace/config/runtime.json").then((nsConfig) => {
			// Looking up the service that we want to talk to.
			const serviceEndpoint = nsConfig.stack_entry
				.map((e) => e.service)
				.flat()
				.find((s) => s.name === "webapi").endpoint;

			// Visit the home page
			cy.visit(`http://${serviceEndpoint}/`);

			// Verify that the page contains the expected text.
			cy.contains("Hello from npm");
		});
	});
});
