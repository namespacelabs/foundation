// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import { cmdData, historyData, makeIcon, monitorData } from "../../ui/icons";
import { Item } from "../../ui/sidebar/Sidebar";
import { useBuildRoute } from "../build/routing";
import { useCommandRoute } from "../command/routing";
import { useTasksRoute } from "../tasks/routing";

export function FooterItems() {
	return (
		<>
			<Build />
			<Command />
			<Tasks />
		</>
	);
}

function Build() {
	const [matches, _] = useBuildRoute();

	return (
		<Item href="/build" icon={makeIcon(monitorData)} active={matches}>
			Build
		</Item>
	);
}

function Command() {
	const [matches, _] = useCommandRoute();

	return (
		<Item href="/command" icon={makeIcon(cmdData)} active={matches}>
			Console
		</Item>
	);
}

function Tasks() {
	const [matches, _] = useTasksRoute();

	return (
		<Item href="/tasks" icon={makeIcon(historyData)} active={matches}>
			Tasks
		</Item>
	);
}
