/*
 * Copyright The OpenTelemetry Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { SpanStatusCode, SpanStatus } from "@opentelemetry/api";
import type * as grpcJsTypes from "@grpc/grpc-js";

// Equivalent to lodash _.findIndex
export const findIndex: <T>(args: T[], fn: (arg: T) => boolean) => number = (
	args,
	fn: Function
) => {
	let index = -1;
	for (const arg of args) {
		index++;
		if (fn(arg)) {
			return index;
		}
	}
	return -1;
};

/**
 * Convert a grpc status code to an opentelemetry SpanStatus code.
 * @param status
 */
export const _grpcStatusCodeToOpenTelemetryStatusCode = (
	status?: grpcJsTypes.status
): SpanStatusCode => {
	if (status !== undefined && status === 0) {
		return SpanStatusCode.UNSET;
	}
	return SpanStatusCode.ERROR;
};

export const _grpcStatusCodeToSpanStatus = (status: number): SpanStatus => {
	return { code: _grpcStatusCodeToOpenTelemetryStatusCode(status) };
};
