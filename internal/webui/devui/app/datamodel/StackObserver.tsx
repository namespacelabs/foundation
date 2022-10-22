// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import React, { useContext, useEffect, useState } from "react";
import { DataType, StackType } from "./Schema";
import { StackSocket } from "./stack";

export const WSContext = React.createContext<StackSocket | null>(null);
const DataContext = React.createContext<DataType | null>(null);

export function ConnectToStack(props: { children: React.ReactNode }) {
	let [conn, _] = useState<StackSocket>(() => {
		const conn = new StackSocket();
		conn.ensureConnected();
		return conn;
	});

	// XXX conn.close() is missing.

	return <WSContext.Provider value={conn}>{props.children}</WSContext.Provider>;
}

export function StackObserver(props: { children: React.ReactNode }) {
	let [data, setData] = useState<DataType | null>(null);
	let ws = useContext(WSContext);

	useEffect(() => {
		return ws?.observeStack((stackUpdate) => {
			// Don't mutate what we got on the wire, for debugging purposes.
			if (stackUpdate) {
				setData({
					...stackUpdate,
					stack: sortStack(stackUpdate.focus || [], stackUpdate.stack),
				});
			}
		});
	}, []);

	return <DataContext.Provider value={data}>{props.children}</DataContext.Provider>;
}

function sortStack(focus: string[], stack?: StackType) {
	// Can sort in place as we just this out of the wire.

	stack?.entry?.sort((a, b) => {
		if (focus.includes(a.server.package_name)) {
			return -1;
		} else if (focus.includes(b.server.package_name)) {
			return 1;
		}

		return a.server.package_name.localeCompare(b.server.package_name);
	});

	return stack;
}

export function useData() {
	return useContext(DataContext);
}
