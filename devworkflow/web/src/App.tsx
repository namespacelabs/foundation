// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { ServerTabs, useCurrentServer } from "./app/server/ServerPanel";
import Sidebar from "./app/sidebar/Sidebar";
import { FooterItems } from "./app/sidebar/Footer";
import TasksPanel from "./app/tasks/TasksPanel";
import { ConnectToStack, StackObserver } from "./datamodel/StackObserver";
import classes from "./app.module.css";
import BuildPanel from "./app/build/BuildPanel";
import CommandPanel from "./app/command/CommandPanel";
import { useLocation, useRoute } from "wouter";
import FullscreenTerminal from "./app/terminal/FullscreenTerminal";
import { useEffect } from "react";
import { useMediaQuery } from "./ui/mediaquery/observe";
import classNames from "classnames";
import Panel from "./ui/panel/Panel";
import ServerInfo from "./app/server/ServerInfo";
import { ItemRow, ItemSpacer } from "./ui/sidebar/Sidebar";
import { Logo } from "./app/logo/Logo";
import { Navbar } from "./app/navbar/Navbar";

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
	let [location, _] = useLocation();
	let currentServer = useCurrentServer();

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
								<div className={classes.emptyPanel}>Select a server to inspect.</div>
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
