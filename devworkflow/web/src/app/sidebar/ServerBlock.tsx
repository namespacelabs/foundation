// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { Link } from "wouter";
import {
	DataType,
	ForwardedPort,
	StackEntryStateType,
	StackEntryType,
} from "../../datamodel/Schema";
import { useTasksByServer } from "../../datamodel/TasksObserver";
import { ExternalLinkIcon } from "../../icons";
import Selectable from "../../ui/sidebar/Selectable";
import Tabs from "../../ui/sidebar/Tabs";
import { useServerRoute } from "../server/routing";
import classes from "./sidebar.module.css";

export default function ServerBlock(props: { data: DataType }) {
	const focusedServers = entriesToServers([props.data.current], props.data.state);
	const supportServers = entriesToServers(
		props.data.stack?.entry?.filter(
			(s) => !s.server.cluster_admin && s.server.id != props.data.current.server.id
		),
		props.data.state
	);

	let tabs = [
		{
			id: "focusedServers",
			label: "Main servers",
			render: () => <ServerList servers={focusedServers} />,
		},
		{
			id: "supportServers",
			label: "Support servers",
			render: () => <ServerList servers={supportServers} />,
		},
	];

	return (
		<div className={classes.serverContent}>
			<Tabs tabs={tabs} />

			<ForwardedPorts data={props.data} />
		</div>
	);
}

interface Server {
	name: string;
	packageName: string;
	id: string;
	lastError?: string;
}

function entriesToServers(
	entries: StackEntryType[] | undefined,
	state: StackEntryStateType[] | undefined
): Server[] {
	return (
		entries?.map((e) => {
			let matchingState = state?.filter((st) => st.package_name === e.server.package_name);
			return {
				id: e.server.id,
				name: e.server.name,
				packageName: e.server.package_name,
				lastError: matchingState?.shift()?.last_error,
			};
		}) || []
	);
}

function ServerList(props: { servers: Server[] }) {
	let [matches, params] = useServerRoute();

	return (
		<>
			{props.servers.map((s) => {
				return (
					<Selectable key={s.packageName} selected={matches && params?.id === s.id}>
						<Server server={s} />
					</Selectable>
				);
			})}
		</>
	);
}

const taskHumanNames: { [key: string]: string } = {
	"graph.compute": "computing",
	"server.build": "building",
	"server.provision": "provisioning",
	"server.deploy": "deploying",
	"server.start": "starting",
};

function humanTaskName(name: string) {
	return taskHumanNames[name] || name;
}

function Server(props: { server: Server }) {
	let runningTask = useTasksByServer(props.server.packageName);

	const parts = props.server.packageName.split("/");
	let p = parts.pop();

	while (parts.length) {
		let would = parts.pop() + "/" + p;
		if (would.length > 34) {
			break;
		}
		p = would;
	}

	if (parts.length) {
		p = "... " + p;
	}

	let badges: string[] = [];

	let isWorking = false;
	if (props.server.lastError) {
		p = "failed: " + props.server.lastError;
	} else if (runningTask.length) {
		// Show the last task, and collapse the rest into "...".
		if (runningTask.length > 1) {
			badges.push("...");
		}

		badges.push(humanTaskName(runningTask[runningTask.length - 1].name));
	}

	return (
		<Link href={`/server/${props.server.id}`}>
			<a className={classes.serverItem}>
				<div className={classes.serverName}>
					<span>{props.server.name}</span>
					{!isWorking &&
						badges.map((b) => (
							<span key={b} className={classes.badge}>
								{b}
							</span>
						))}
				</div>
				<div className={classes.serverPackageName}>
					{isWorking ? (
						badges.map((b) => (
							<span key={b} className={classes.badge}>
								{b}
							</span>
						))
					) : (
						<span>{p}</span>
					)}
				</div>
			</a>
		</Link>
	);
}

function ForwardedPorts(props: { data: DataType }) {
	if (!props.data.forwarded_port) {
		return null;
	}

	let tabs = [
		{
			id: "ports",
			label: "Ports",
			render: () => (
				<>
					{sortPorts(props.data.current.server.package_name, props.data.forwarded_port).map((p) => (
						<Port key={p.container_port} data={props.data} p={p} />
					))}
				</>
			),
		},
	];

	return <Tabs tabs={tabs} />;
}

function sortPorts(current: string, ports?: ForwardedPort[]) {
	let copy = [...(ports || [])];

	copy.sort((a, b) => {
		let apkg = a.endpoint.server_owner || "<stack>";
		let bpkg = b.endpoint.server_owner || "<stack>";

		if (apkg === current) {
			return -1;
		} else if (bpkg === current) {
			return 1;
		}

		if (apkg === bpkg) {
			return a.container_port - b.container_port;
		}
		return apkg.localeCompare(bpkg);
	});

	return copy;
}

function Port(props: { data: DataType; p: ForwardedPort }) {
	let { p } = props;
	let serverName = p.endpoint.server_owner;

	for (let s of props.data.stack?.entry || []) {
		if (s.server.package_name === p.endpoint.server_owner) {
			serverName = s.server.name;
			break;
		}
	}

	let body = (
		<>
			<div className={classes.serviceDesc}>
				<span className={classes.ports}>{p.local_port}</span>{" "}
				<span className={classes.serviceName}>{serverName} </span>
				<ExternalLinkIcon />
			</div>
			<div className={classes.serviceDetails}>
				{p.endpoint.port?.name || p.endpoint.service_name} ({p.container_port})
			</div>
		</>
	);

	return (
		<a
			className={classes.port}
			href={`${window.location.protocol}//${window.location.hostname}:${p.local_port}`}
			target="_blank">
			{body}
		</a>
	);
}
