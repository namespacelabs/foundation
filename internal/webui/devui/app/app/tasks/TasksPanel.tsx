// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import { useTasksRoute } from "./routing";
import Task from "./Task";
import TaskList from "./TaskList";
import classes from "./tasks.module.css";

export default function TasksPanel() {
	const [match, params] = useTasksRoute();

	if (match) {
		return (
			<div className={classes.taskPanel}>
				{params?.id ? <Task id={params.id} what={params.what} /> : <TaskList />}
			</div>
		);
	}

	return null;
}
