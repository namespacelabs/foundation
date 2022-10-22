// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import { formatParsedDur, formatParsedTime } from "../../app/tasks/time";
import { Segment } from "./Segment";

type BuildEvent = {
	digest: string;
	name?: string;
	parts: Segment[];
	started?: number;
	startedStr?: string;
	completed?: number;
	durationStr?: string;
	cached?: boolean;
	statuses: Status[];
};

type Status = {
	id: string;
	parts: Segment[];
	started?: number;
	startedStr?: string;
	completed?: number;
	durationStr?: string;
};

export type WireEvent = {
	s: string;
	e?: any;
	started?: string;
	completed?: string;
};

const imageRegexp = new RegExp("(docker-image://)?(docker.io/library/([^ $]+))");

export class BuildInvocation {
	readonly id: string;
	readonly started: number;
	completed?: number;

	private events: { [digest: string]: BuildEvent } = {};
	private digests: string[] = [];

	constructor(id: string, started: number) {
		this.id = id;
		this.started = started;
	}

	parse(ev: WireEvent) {
		let e = ev.e || {};

		for (let v of e.Vertexes || []) {
			let digest: string = v.Digest;
			let b: BuildEvent;

			if (this.events[digest]) {
				b = this.events[digest];
			} else {
				b = { digest: digest, statuses: [], parts: [] };
				this.events[digest] = b;
				this.digests.push(digest);
			}

			parseDates(b, v);
			b.cached = !!v.Cached;
			let name: string = v.Name;
			if (name && name !== b.name) {
				b.name = name;
				b.parts = parseParts(name);
			}
		}

		for (let v of e.Statuses || []) {
			let vertexId: string = v.Vertex;
			let id: string | undefined = v.ID;
			if (!id || !this.events[vertexId]) {
				// Dropping;
				continue;
			}
			let vertex = this.events[vertexId];
			// XXX expensive
			let filtered = vertex.statuses.filter((s) => s.id === id);
			let s: Status;
			if (!filtered.length) {
				s = { id: id, parts: parseParts(id) };
				vertex.statuses.push(s);
			} else {
				s = filtered[0];
			}
			parseDates(s, v);
		}

		if (ev.completed) {
			this.completed = Date.parse(ev.completed);
		}
	}

	tidy() {
		this.digests.sort((a: string, b: string) => {
			let aev = this.events[a],
				bev = this.events[b];

			if (aev.started && bev.started) {
				return aev.started - bev.started;
			} else if (aev.started) {
				return -1;
			} else if (aev.started) {
				return 1;
			} else {
				return 0;
			}
		});
	}

	sorted() {
		return this.digests.map((d) => this.events[d]);
	}
}

function parseParts(name: string) {
	let parts = [];
	while (name.length) {
		let m = name.match(imageRegexp);
		if (!m) {
			parts.push({ t: name, k: "text" });
			break;
		}

		if (m.index) {
			parts.push({ t: name.substring(0, m.index), k: "text" });
			name = name.substring(m.index);
		}

		if (m[3]) {
			parts.push({ t: m[3], k: "image" });
			name = name.substring(m[0].length);
		}
	}
	return parts;
}

function parseDates(
	dst: {
		started?: number;
		startedStr?: string;
		completed?: number;
		durationStr?: string;
	},
	src: { Started?: string; Completed?: string }
) {
	if (src.Started) {
		dst.started = Date.parse(src.Started);
		dst.startedStr = formatParsedTime(new Date(dst.started));
	}
	if (src.Completed) {
		dst.completed = Date.parse(src.Completed);
		if (dst.started) {
			dst.durationStr = formatParsedDur(dst.completed - dst.started);
		}
	}
}
