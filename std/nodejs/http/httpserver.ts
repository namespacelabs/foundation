// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// Referring to "middie" because it declares a method "use" on "FastifyInstance".
// "use" is needed for middleware.
/// <reference types="middie"/>
import { FastifyInstance } from "fastify";
import "source-map-support/register";

export interface HttpServer {
	fastify(): FastifyInstance;
}
