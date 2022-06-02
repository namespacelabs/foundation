// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { SpanProcessor } from "@opentelemetry/sdk-trace-base";

export interface Exporter {
	addSpanProcessor(spanProcessor: SpanProcessor): void;
}
