// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import React, { useEffect, useRef } from "react";
import { XTerm } from "xterm-for-react";
import { FitAddon } from "xterm-addon-fit";
import { WebLinksAddon } from "xterm-addon-web-links";
import classes from "./terminal.module.css";

export default function Terminal(props: {
	children: (termRef: React.RefObject<XTerm>) => void;
	onData?: (data: string) => void;
	onResize?: (evt: { cols: number; rows: number }) => void;
}) {
	let termRef = useRef<XTerm>(null);
	let fitAddonRef = useRef(new FitAddon());
	let observer = useRef(
		new ResizeObserver(() => {
			fitAddonRef.current.fit();
		})
	);

	useEffect(() => {
		fitAddonRef.current.fit();

		if (termRef.current?.terminalRef.current) {
			let elem = termRef.current.terminalRef.current;
			observer.current.observe(elem);
			return () => {
				observer.current.unobserve(elem);
			};
		}
	});

	props.children(termRef);

	return (
		<XTerm
			onData={props.onData}
			onResize={props.onResize}
			ref={termRef}
			className={classes.t}
			options={{
				convertEol: true,
				disableStdin: !props.onData,
				rows: 4,
			}}
			addons={[fitAddonRef.current, new WebLinksAddon()]}
		/>
	);
}
