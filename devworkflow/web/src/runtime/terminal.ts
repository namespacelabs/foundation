// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { BytesSocket } from "../api/websocket";

export class TerminalSocket extends BytesSocket {
  constructor(args: {
    kind: string;
    apiUrl: string;
    setConnected?: (connected: boolean) => void;
  }) {
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