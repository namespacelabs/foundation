// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { InfoLevel, Logger } from "./logger";

export abstract class Socket {
	private conn?: WebSocket;
	private reconnectTimer?: number;
	protected readonly logger: Logger;
	private readonly apiUrl: string;
	private readonly setConnected?: (connected: boolean) => void;
	private readonly autoReconnect: boolean;

	constructor(args: {
		kind: string;
		apiUrl: string;
		setConnected?: (connected: boolean) => void;
		autoReconnect: boolean;
	}) {
		this.logger = new Logger(args.kind, InfoLevel);
		this.apiUrl = args.apiUrl;
		this.setConnected = args.setConnected;
		this.autoReconnect = args.autoReconnect;
	}

	private connect(timeout: number) {
		const conn = new WebSocket(`ws://${window.location.host}/ws/fn/${this.apiUrl}`);

		conn.addEventListener("open", (evt) => {
			this.logger.info("connected", evt);
			timeout = 0; // Managed to connect, next time try to reconnect quickly.
			if (this.setConnected) this.setConnected(true);
		});

		conn.addEventListener("message", (evt) => {
			this.onMessage(evt.data);
		});

		conn.addEventListener("close", (evt) => {
			this.logger.info("connection was closed");
			this.conn = undefined;
			if (this.setConnected) this.setConnected(false);

			if (this.autoReconnect) {
				this.ensureConnected(timeout);
			}
		});

		conn.addEventListener("error", (evt) => {
			this.logger.error("error", evt);
			try {
				this.conn?.close();
			} finally {
				this.conn = undefined;
				if (this.autoReconnect) {
					this.ensureConnected(timeout);
				}
			}
		});

		this.conn = conn;
	}

	close() {
		if (this.conn) {
			this.logger.info("closed websocket");
			this.conn.close();
			this.conn = undefined;
		}

		if (this.reconnectTimer) {
			this.logger.info("cancelled reconnect");
			clearTimeout(this.reconnectTimer);
			this.reconnectTimer = undefined;
		}
	}

	ensureConnected(timeout: number = 0) {
		this.logger.info(
			"ensureConnected",
			{ timeout },
			{ conn: this.conn, reconnectTimer: this.reconnectTimer }
		);

		if (this.conn) {
			return;
		}

		if (this.reconnectTimer) {
			return;
		}

		const newTimeout = timeout ? Math.min(10000, timeout * 2) : 250;
		this.logger.info("will connect in", timeout, "next", newTimeout);

		this.reconnectTimer = setTimeout(() => {
			this.reconnectTimer = undefined;
			this.connect(newTimeout);
		}, timeout);
	}

	protected sendJson(input: any) {
		this.logger.debug("send", { input });
		this.conn?.send(JSON.stringify(input));
	}

	protected abstract onMessage(buf: any): void;
}

export class BytesSocket extends Socket {
	private observers: ((t: ArrayBuffer) => void)[] = [];

	protected override async onMessage(data: any) {
		let message = await data.arrayBuffer();

		this.observers.forEach((f) => f(message));
	}

	observe(observer: (data: ArrayBuffer) => void) {
		this.observers.push(observer);

		return () => {
			this.observers = this.observers.filter((v) => v != observer);
		};
	}
}
