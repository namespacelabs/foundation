// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

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
					stack: sortStack(stackUpdate.current.server.package_name, stackUpdate.stack),
				});
			}
		});
	}, []);

	return <DataContext.Provider value={data}>{props.children}</DataContext.Provider>;
}

function sortStack(current: string, stack?: StackType) {
	// Can sort in place as we just this out of the wire.

	stack?.entry?.sort((a, b) => {
		if (a.server.package_name == current) {
			return -1;
		} else if (b.server.package_name == current) {
			return 1;
		}

		return a.server.package_name.localeCompare(b.server.package_name);
	});

	return stack;
}

export function useData() {
	return useContext(DataContext);
}
