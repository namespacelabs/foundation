// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import { useContext } from "react";
import { useLocation } from "wouter";
import { WSContext } from "../../datamodel/StackObserver";
import Button from "../../ui/button/Button";

export function RebuildButton() {
	let ws = useContext(WSContext);
	let [_, setLocation] = useLocation();

	return (
		<Button
			onClick={() => {
				if (ws) {
					ws.send({ reloadWorkspace: true });
					setLocation(`/server/7hzne001dff2rpdxav703bwqwc/command`);
				}
			}}>
			Rebuild 🐞
		</Button>
	);
}

export function NewTerminalButton(props: { id: string }) {
	return (
		<Button
			onClick={() => {
				window.open(`/terminal/${props.id}`, "_blank");
			}}>
			New Terminal
		</Button>
	);
}
