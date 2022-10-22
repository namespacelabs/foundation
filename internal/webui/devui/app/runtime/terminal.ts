// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import { BytesSocket } from "../api/websocket";

export class TerminalSocket extends BytesSocket {
	constructor(args: { kind: string; apiUrl: string; setConnected?: (connected: boolean) => void }) {
		super({
			kind: args.kind,
			apiUrl: args.apiUrl,
			setConnected: args.setConnected,
			autoReconnect: false,
		});
	}

	send(input: { stdin?: string; resize?: { width: number; height: number } }) {
		this.sendJson(input);
	}
}
