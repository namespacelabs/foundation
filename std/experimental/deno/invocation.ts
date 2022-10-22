// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

export async function readRequest() {
	const buf = new Uint8Array(1024);

	const n = await Deno.stdin.read(buf);
	if (!n) {
		throw new Error("missing input");
	} else {
		const raw = new TextDecoder().decode(buf.subarray(0, n));
		return JSON.parse(raw);
	}
}

interface InvocationResponse {
	resource?: Resource;
}

interface Resource {
	contents: string;
}

export async function respond(value: InvocationResponse) {
	const encoder = new TextEncoder();
	await Deno.stdout.write(encoder.encode(JSON.stringify(value)));
}
