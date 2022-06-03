// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";
import {GrpcRegistrar} from "@namespacelabs.dev-foundation/std-nodejs-grpc"
import * as i0 from "@namespacelabs.dev-foundation/std-nodejs-monitoring-tracing/types_pb";
import * as i1 from "@namespacelabs.dev-foundation/std-nodejs-monitoring-tracing/exporter";



export const Package = {
	name: "namespacelabs.dev/foundation/std/nodejs/monitoring/tracing",
};

const initializer = {
	package: Package,
	initialize: impl.initialize,
};

export type Prepare = () => Promise<void> | void;
export const prepare: Prepare = impl.initialize;

export const TransitiveInitializers: Initializer[] = [
	initializer,
];


export const ExporterProvider = (graph: DependencyGraph, input: i0.ExporterArgs) =>
	provideExporter(
		input
	);

export type ProvideExporter = (input: i0.ExporterArgs) =>
		i1.Exporter;
export const provideExporter: ProvideExporter = impl.provideExporter;
