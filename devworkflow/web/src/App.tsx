// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { Logo } from "@namespacelabs.dev/webui-components/logo/Logo";
import classNames from "classnames";
import { useEffect } from "react";
import { useRoute } from "wouter";
import classes from "./app.module.css";
import BuildPanel from "./app/build/BuildPanel";
import CommandPanel from "./app/command/CommandPanel";
import { Navbar } from "./app/navbar/Navbar";
import ServerInfo from "./app/server/ServerInfo";
import { ServerTabs, useCurrentServer } from "./app/server/ServerPanel";
import { FooterItems } from "./app/sidebar/Footer";
import Sidebar from "./app/sidebar/Sidebar";
import TasksPanel from "./app/tasks/TasksPanel";
import FullscreenTerminal from "./app/terminal/FullscreenTerminal";
import { ConnectToStack, StackObserver, useData } from "./datamodel/StackObserver";
import { useMediaQuery } from "./ui/mediaquery/observe";
import Panel from "./ui/panel/Panel";
import { ItemRow, ItemSpacer } from "./ui/sidebar/Sidebar";
import TerminalTabs from "./ui/termchrome/TerminalTabs";
import Terminal from "./ui/terminal/Terminal";

export default function App() {
	let [match, params] = useRoute("/terminal/:id");

	useEffect(() => {
		document.body.classList.remove("terminal");
		if (match) {
			document.body.classList.add("terminal");
		}
	});
	if (match) {
		if (!params?.id) {
			return <div>bad request</div>;
		}

		return <FullscreenTerminal id={params.id} />;
	}

	return (
		<ConnectToStack>
			<StackObserver>
				<Contents />
			</StackObserver>
		</ConnectToStack>
	);
}

function Contents() {
	let currentServer = useCurrentServer();
	let data = useData();

	let isBigScreen = useMediaQuery("screen and (min-width: 1100px)");

	return (
		<>
			<Navbar></Navbar>
			<div
				id="content"
				className={classNames({
					inlineSidebar: true,
					hasFooter: true,
				})}>
				<Panel>
					<div className="fiddle">
						<Sidebar fixed={false} />
						<Panel>
							{currentServer ? (
								<>
									<ServerInfo {...currentServer} />
									{isBigScreen ? <ServerTabs {...currentServer} /> : null}
								</>
							) : (
								<div className={classes.portForwardingPanel}>
									<RenderPortForwarding raw={data?.rendered_port_forwarding} />
								</div>
							)}
						</Panel>
					</div>
					{isBigScreen || !currentServer ? null : <ServerTabs {...currentServer} />}
					<BuildPanel />
					<CommandPanel />
					<TasksPanel />
				</Panel>
			</div>
			<ItemRow>
				<FooterItems />
				<ItemSpacer />
				<Logo />
			</ItemRow>{" "}
		</>
	);
}

function RenderPortForwarding(props: { raw: string | undefined }) {
	return (
		<>
			<TerminalTabs
				tabs={[{ what: "servers", label: "Services exported" }]}
				current="servers"></TerminalTabs>
			<div className={classes.terminalWrapper}>
				<Terminal>
					{(termRef) => {
						useEffect(() => {
							termRef.current?.terminal.clear();
							if (props.raw) {
								termRef.current?.terminal.write(props.raw);
							}
						}, [props.raw]);
					}}
				</Terminal>
			</div>
		</>
	);
}
