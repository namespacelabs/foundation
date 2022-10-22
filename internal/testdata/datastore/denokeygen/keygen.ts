// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import { encode } from "https://deno.land/std@0.147.0/encoding/base64.ts";
import {
	readRequest,
	respond,
} from "https://namespacelabs.dev/foundation/std/experimental/deno/invocation.ts";

await readRequest();

respond({
	resource: {
		contents: encode(crypto.getRandomValues(new Uint8Array(128))),
	},
});
