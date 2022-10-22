// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import React from "react";
import { StreamOutput } from "../../ui/terminal/StreamOutput";
import { OutputSocket } from "../../devworkflow/output";

export default function StreamSocket(props: { what: string }) {
	return (
		<StreamOutput
			makeSocket={() =>
				new OutputSocket({
					endpoint: props.what,
				})
			}
		/>
	);
}
