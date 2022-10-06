// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { useEffect, useRef } from "react";
import { TerminalSocket } from "../../runtime/terminal";
import Terminal from "../../ui/terminal/Terminal";

export default function ServerTerminal(props: {
	serverId: string;
	what: string;
	wireStdin?: boolean;
}) {
	let wsRef = useRef<TerminalSocket | null>(null);

	return (
		<Terminal
			onData={
				props.wireStdin
					? (data: string) => {
							wsRef.current?.send({ stdin: window.btoa(data) });
					  }
					: undefined
			}
			onResize={
				props.wireStdin
					? (evt) => {
							wsRef.current?.send({
								resize: { width: evt.cols, height: evt.rows },
							});
					  }
					: undefined
			}>
			{(termRef) => {
				useEffect(() => {
					wsRef.current = new TerminalSocket({
						kind: props.what,
						apiUrl: `server/${props.serverId}/${props.what}`,
					});

					wsRef.current.ensureConnected();

					let firstBuffer = true;
					const release = wsRef.current.observe((buffer) => {
						if (termRef.current) {
							if (firstBuffer) {
								termRef.current.terminal.clear();
								firstBuffer = false;

								// At start, resize the terminal to the current xterm size.
								wsRef.current?.send({
									resize: {
										width: termRef.current.terminal.cols,
										height: termRef.current.terminal.rows,
									},
								});
							}
							termRef.current.terminal.write(new Uint8Array(buffer));
						}
					});

					return () => {
						release();
						wsRef.current?.close();
						wsRef.current = null;
					};
				}, [props.serverId]);
			}}
		</Terminal>
	);
}
