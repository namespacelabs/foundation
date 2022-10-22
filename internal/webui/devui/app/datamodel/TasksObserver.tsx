// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import { useContext, useEffect, useState } from "react";
import { Task } from "./stack";
import { WSContext } from "./StackObserver";

export function useTasks() {
	let [data, setData] = useState<Task[]>([]);
	let ws = useContext(WSContext);

	useEffect(() => {
		return ws?.observeTasks((tasks) => {
			setData(tasks);
		});
	}, []);

	return data;
}

export function useTasksByServer(server: string) {
	let tasks = useTasks();

	return tasks.filter((t) => !t.completed_ts && (t.scope || []).indexOf(server) >= 0);
}
