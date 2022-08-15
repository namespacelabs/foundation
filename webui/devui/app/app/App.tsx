// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { Chrome, Navbar } from "@namespacelabs.dev/webui-components/chrome/Chrome";
import { Logo } from "@namespacelabs.dev/webui-components/logo/Logo";
import { useEffect } from "react";
import { useRoute } from "wouter";
import classes from "./app.module.css";
import BuildPanel from "./build/BuildPanel";
import CommandPanel from "./command/CommandPanel";
import ServerInfo from "./server/ServerInfo";
import { ServerTabs, useCurrentServer } from "./server/ServerPanel";
import { FooterItems } from "./sidebar/Footer";
import Sidebar from "./sidebar/Sidebar";
import TasksPanel from "./tasks/TasksPanel";
import FullscreenTerminal from "./terminal/FullscreenTerminal";
import { ConnectToStack, StackObserver, useData } from "../datamodel/StackObserver";
import Panel from "../ui/panel/Panel";
import { ItemRow, ItemSpacer } from "../ui/sidebar/Sidebar";
import TerminalTabs from "../ui/termchrome/TerminalTabs";
import Terminal from "../ui/terminal/Terminal";

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

	return (
		<Chrome
			headerLabel="Development UI"
			footer={
				<>
					<BuildPanel />
					<CommandPanel />
					<TasksPanel />
					<ItemRow>
						<FooterItems />
						<ItemSpacer />
						<Logo />
					</ItemRow>
				</>
			}>
			<Navbar>
				<Sidebar fixed={false} />
			</Navbar>
			{currentServer ? (
				<Panel>
					<ServerInfo {...currentServer} />
					<ServerTabs {...currentServer} />
				</Panel>
			) : (
				<div className={classes.portForwardingPanel}>
					<RenderPortForwarding raw={data?.rendered_port_forwarding} />
				</div>
			)}
		</Chrome>
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
