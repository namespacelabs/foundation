// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { Socket } from "../api/websocket";
import { DataType } from "./Schema";

export type Task = {
	id: string;
	name: string;
	human_readable_label?: string;
	created_ts: string;
	completed_ts?: string;
	error_message?: string;
	output?: {
		name: string;
		content_type: string;
	}[];
	scope?: string[];
	env_name?: string;
	cached?: boolean;
};

type Update = {
	stack_update?: DataType;
	task_update?: Task[];
};

type TaskUpdateObserverFunc = (tasks: Task[]) => void;
type StackUpdateObserverFunc = (stack: DataType) => void;

type TaskUpdateIndex = { [key: string]: Task };

export class StackSocket extends Socket {
	// List of observers that are notified of updates to the instances below.
	private tasksObs: TaskUpdateObserverFunc[] = [];
	private stackObs: StackUpdateObserverFunc[] = [];

	// Accumulated from the latest payload.
	private tasks: Task[] = [];
	private taskIndex: TaskUpdateIndex = {};
	private latestStack: DataType | null = null;

	constructor() {
		super({ kind: "stack", apiUrl: "stack", autoReconnect: true });
	}

	protected override async onMessage(data: any) {
		this.onStackMessage(JSON.parse(await data.text()));
	}

	private onStackMessage(message: Update) {
		this.logger.debug("message", message);

		if (message.stack_update) {
			this.latestStack = message.stack_update;
			let latestStack = this.latestStack;
			if (latestStack) {
				this.stackObs.forEach((f) => f(latestStack));
			}
		}

		if (message.task_update) {
			let sortNeeds = 0;

			message.task_update.forEach((update) => {
				if (!update.id) {
					return;
				}

				if (this.taskIndex[update.id]) {
					if (this.taskIndex[update.id].created_ts != update.created_ts) {
						sortNeeds++;
					}
					Object.assign(this.taskIndex[update.id], update);
				} else {
					this.tasks.push(update);
					this.taskIndex[update.id] = update;
					sortNeeds++;
				}
			});

			if (sortNeeds) {
				// Keeping it simple. Would favor a btree-type treatment at some point.
				this.tasks.sort((a, b) => {
					return parseInt(a.created_ts) - parseInt(b.created_ts);
				});
			}

			this.tasksObs.forEach((f) => f(this.tasks));
		}
	}

	send(msg: {
		reloadWorkspace?: boolean;
		setWorkspace?: { absRoot: string; packageName: string; envName: string };
	}) {
		this.sendJson(msg);
	}

	observeStack(observer: StackUpdateObserverFunc) {
		this.stackObs.push(observer);

		if (this.latestStack) {
			observer(this.latestStack);
		}

		return () => {
			this.stackObs = this.stackObs.filter((v) => v != observer);
		};
	}

	observeTasks(observer: TaskUpdateObserverFunc) {
		this.tasksObs.push(observer);

		observer(this.tasks);

		return () => {
			this.tasksObs = this.tasksObs.filter((v) => v != observer);
		};
	}
}
