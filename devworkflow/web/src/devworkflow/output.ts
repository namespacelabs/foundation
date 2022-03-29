// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { Logger } from "../api/logger";

type ObserverFunc = (data: ArrayBuffer) => void;

export class OutputSocket {
  private readonly endpoint: string;
  private readonly logger;
  private conn?: WebSocket;
  private reconnectTimer?: number;
  private observers: ObserverFunc[] = [];
  private readonly setConnected: (connected: boolean) => void;

  constructor(args: {
    endpoint: string;
    setConnected?: (connected: boolean) => void;
  }) {
    this.logger = new Logger(`build.${args.endpoint}`);
    this.endpoint = args.endpoint;
    this.setConnected = args.setConnected || ((_: boolean) => {});
  }

  private connect(timeout: number) {
    const conn = new WebSocket(
      `ws://${window.location.host}/ws/fn/${this.endpoint}`
    );

    conn.addEventListener("open", (evt) => {
      this.logger.info("connected", evt);
      timeout = 0; // Managed to connect, next time try to reconnect quickly.
      this.setConnected(true);
    });

    conn.addEventListener("message", async (evt) => {
      this.onMessage(await evt.data.arrayBuffer());
    });

    conn.addEventListener("close", (evt) => {
      this.logger.info("connection was closed");
      this.conn = undefined;
      this.setConnected(false);
    });

    conn.addEventListener("error", (evt) => {
      console.error(`[build.${this.endpoint}]`, evt);
      try {
        this.conn?.close();
      } finally {
        this.conn = undefined;
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

  private onMessage(message: ArrayBuffer) {
    this.observers.forEach((f) => f(message));
  }

  observe(observer: ObserverFunc) {
    this.observers.push(observer);

    return () => {
      this.observers = this.observers.filter((v) => v != observer);
    };
  }
}