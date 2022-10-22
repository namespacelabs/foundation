// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import classNames from "classnames";
import {
	EnvironmentType,
	ExportedServiceType,
	NodeKindEnum,
	NodeType,
	ServerType,
	StackEntryType,
	StackType,
} from "../../datamodel/Schema";
import { useTasks } from "../../datamodel/TasksObserver";
import { ExternalLinkIcon } from "../../ui/icons";
import Button from "../../ui/button/Button";
import { useMediaQuery } from "../../ui/mediaquery/observe";
import { TaskLink } from "../tasks/TaskList";
import { formatDur } from "../tasks/time";
import { NewTerminalButton, RebuildButton } from "./Buttons";
import { PackageLink } from "./PackageLink";
import classes from "./server.module.css";

export default function ServerInfo(props: {
	env: EnvironmentType;
	stack: StackType;
	server: StackEntryType;
}) {
	let isBigScreen = useMediaQuery("screen and (min-width: 800px)");

	let { server, node } = props.server;
	let endpoints = endpointsOf(props.stack, server.package_name);
	let services = node?.filter((n) => n.kind == NodeKindEnum.SERVICE) || [];

	const grpcUrlCommand = (svc: NodeType) => {
		const host = window.location.hostname;
		const serviceName = svc.export_service![0].proto_typename;
		const port = endpoints.filter((e) => svc.package_name.endsWith(e.service_name))[0]!.port
			?.container_port;

		return `ns tools grpcurl -plaintext ${host}:${port} list ${serviceName}`;
	};

	return (
		<div className={classes.scrollOk}>
			<div className={classes.serverInfo}>
				<div>
					<div className={classes.serverNameWrapper}>
						{isBigScreen ? (
							<>
								<div className={classes.serverName}>{server.name}</div>
							</>
						) : null}
						<PackageLink
							packageName={server.package_name}
							collapsable={true}
							className={classNames({
								[classes.highlight]: !isBigScreen,
							})}
						/>
					</div>
					<div>
						<RebuildButton />
						<NewTerminalButton id={server.id} />
					</div>
				</div>

				<div className={classNames(classes.row, classes.serverRow)}>
					<div className={classes.infoBlock}>
						<div className={classes.blockHeader}>Services</div>
						{services.length ? (
							services.map((svc) => (
								<div key={svc.package_name} className={classes.service}>
									<PackageLink packageName={svc.package_name} collapsable={true} />
									{svc.export_service?.length && svc.export_service[0].proto.length ? (
										<div className={classNames(classes.row, classes.protoRow)}>
											&#8627;{" "}
											<ExpandableProto service={svc.export_service[0]}>
												{({ last, service }) => (
													<FileLink packageName={svc.package_name} filename={service.proto[0]}>
														{last}
													</FileLink>
												)}
											</ExpandableProto>
											<Button
												compact={true}
												title={grpcUrlCommand(svc)}
												onClick={() => navigator.clipboard.writeText(grpcUrlCommand(svc))}>
												Copy `grpcurl` command
											</Button>
										</div>
									) : null}
								</div>
							))
						) : (
							<div>&mdash;</div>
						)}

						<div className={classNames(classes.blockHeader)}>Endpoints</div>
						{endpoints.length ? (
							endpoints.map((endpoint) =>
								endpoint.port ? (
									<div key={endpoint.allocated_name} className={classes.endpoint}>
										<div>
											{endpoint.service_name}{" "}
											<span className={classes.protocols}>
												({endpoint.service_metadata?.map((md) => md.protocol).join(" ")})
											</span>
										</div>
										<div className={classes.portRow}>
											<span className={classes.label}>Port:</span> {endpoint.port.container_port} (
											{endpoint.type})
										</div>
									</div>
								) : null
							)
						) : (
							<div>&mdash;</div>
						)}

						<div className={classNames(classes.blockHeader)}>Tasks</div>
						<ServerTasks
							env={props.env}
							packageName={server.package_name}
							tasks={[{ name: "server.build", label: "Last build" }]}
						/>
					</div>

					<div className={classes.infoBlock}>
						<div className={classes.blockHeader}>Dependencies</div>

						{props.server.server.user_imports?.map((dep) => (
							<PackageLink key={dep} packageName={dep} collapsable={true} secondary={false} />
						))}

						{props.server.server.import
							?.filter((dep) => !isUserImport(props.server.server, dep))
							.map((dep) => (
								<PackageLink
									key={dep}
									packageName={dep}
									collapsable={true}
									secondary={!isUserImport(props.server.server, dep)}
								/>
							))}

						{(props.server.server.user_imports?.length || 0) +
							(props.server.server.import?.length || 0) ===
						0 ? (
							<div>&mdash;</div>
						) : null}
					</div>
				</div>
			</div>
		</div>
	);
}

function isUserImport(server: ServerType, dep: string) {
	if (server.user_imports) {
		return server.user_imports.indexOf(dep) >= 0;
	}

	return false;
}

function endpointsOf(stack: StackType, pkg: string) {
	return (stack.endpoint || []).filter((e) => e.server_owner === pkg);
}

type TaskToRender = {
	name: string;
	label: string;
};

function ServerTasks(props: { env: EnvironmentType; packageName: string; tasks: TaskToRender[] }) {
	let tasks = useTasks();

	return (
		<>
			{props.tasks.map((t) => {
				let matching = tasks.filter(
					({ name, env_name, scope, completed_ts: completedTs }) =>
						env_name === props.env.name &&
						name === t.name &&
						completedTs &&
						(scope || []).indexOf(props.packageName) >= 0
				);
				if (!matching.length) {
					return <div key={t.label}>{t.label}: &mdash;</div>;
				}

				let last = matching[matching.length - 1];

				return (
					<div key={t.label}>
						{t.label}:{" "}
						<TaskLink task={last}>
							took {formatDur(last.created_ts, last.completed_ts || "")}
						</TaskLink>
					</div>
				);
			})}
		</>
	);
}

function FileLink(props: { packageName: string; filename: string; children: React.ReactNode }) {
	let { packageName, filename } = props;
	let parts = packageName.split("/");

	if (!packageName.startsWith("github.com/") || parts.length < 4) {
		return <div className={classes.dep}>{props.children}</div>;
	}

	let content = parts.splice(3);
	content.push(filename);
	// XXX `main` is not universal.
	let url = "https://" + parts.join("/") + "/blob/main/" + content.join("/");

	return (
		<a href={url} target="_blank" className={classes.dep}>
			{props.children} <ExternalLinkIcon />
		</a>
	);
}

function ExpandableProto(props: {
	service: ExportedServiceType;
	children: (args: { service: ExportedServiceType; last: string }) => JSX.Element;
}) {
	let parts = props.service.proto_typename.split(".");
	let last = parts.splice(parts.length - 1);

	return (
		<>
			<span className={classes.prefix}>{parts.map((p) => p[0]).join(".")}</span>
			{props.children({ last: last[0], service: props.service })}
		</>
	);
}
