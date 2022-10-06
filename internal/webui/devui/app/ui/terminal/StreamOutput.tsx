// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { useEffect, useRef } from "react";
import { OutputSocket } from "../../devworkflow/output";
import Terminal from "./Terminal";
import classes from "./terminal.module.css";

export function StreamOutput(props: { makeSocket: () => OutputSocket }) {
	return (
		<div className={classes.fullscreenWrapper}>
			<Terminal>
				{(termRef) => {
					useEffect(() => {
						const conn = props.makeSocket();

						conn.ensureConnected();

						let firstBuffer = true;
						const release = conn.observe((buffer) => {
							if (termRef.current) {
								if (firstBuffer) {
									termRef.current.terminal.clear();
									firstBuffer = false;
								}
								termRef.current.terminal.write(new Uint8Array(buffer));
							}
						});

						return () => {
							release();
							conn.close();
						};
					}, []);
				}}
			</Terminal>
		</div>
	);
}
