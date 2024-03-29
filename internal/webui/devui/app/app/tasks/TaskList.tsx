// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import React from "react";
import { Link } from "wouter";
import { Task } from "../../datamodel/stack";
import { useTasks } from "../../datamodel/TasksObserver";
import { ServerLink } from "../server/PackageLink";
import classes from "./tasks.module.css";
import { formatDur, formatTime } from "./time";

export default function TaskList() {
	let tasks = useTasks();
	console.log(tasks);
	return (
		<div className={classes.taskGrid}>
			{tasks.map((t) => (
				<React.Fragment key={t.id}>
					<div className={classes.taskStart}>{formatTime(t.created_ts)}</div>
					<div className={classes.taskWhat}>
						<TaskLink task={t}>{t.name}</TaskLink>
					</div>
					<div className={classes.taskArgs}>
						{t.scope?.map((p) => (
							<ServerLink key={p} packageName={p} />
						))}
					</div>
					<div className={classes.taskEnd}>
						{t.cached ? "cached" : t.completed_ts ? formatDur(t.created_ts, t.completed_ts) : null}
					</div>
				</React.Fragment>
			))}
		</div>
	);
}

export function TaskLink(props: { task: Task; children: React.ReactNode }) {
	let { task } = props;
	let outputs = (task.output || []).filter(
		(o) => o.content_type === "application/json+fn.buildkit"
	);
	let o: string;

	if (outputs.length) {
		o = `${task.id}/${outputs[0].name}`;
	} else if (props.task.output?.length) {
		o = `${task.id}/${props.task.output[0].name}`;
	} else {
		o = task.id;
	}

	return (
		<Link href={`/tasks/${o}`}>
			<a className={classes.serverLink}>{props.children}</a>
		</Link>
	);
}
