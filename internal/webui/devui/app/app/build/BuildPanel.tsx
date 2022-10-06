// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import BuildKitLog from "../../ui/buildkit/BuildkitLog";
import TerminalTabs from "../../ui/termchrome/TerminalTabs";
import { TerminalWrapper } from "../../ui/termchrome/TerminalWrapper";
import StreamSocket from "../server/StreamSocket";
import { useBuildRoute } from "./routing";

export default function BuildPanel() {
	const [match, params] = useBuildRoute();

	if (!match) {
		return null;
	}

	let tabs = [
		{ what: "json", label: "BuildKit" },
		{ what: "output", label: "Output" },
	];

	type TabProps = { what: string };

	let constructors: { [key: string]: (tabProps: TabProps) => JSX.Element } = {
		json: (_: TabProps) => <BuildKitLog apiUrl="build.json" />,
		output: (_: TabProps) => <StreamSocket key="build/output" what="build" />,
	};

	let current = params?.what || tabs[0].what;

	return (
		<TerminalWrapper>
			<TerminalTabs tabs={tabs} makeHref={(what: string) => `/build/${what}`} current={current} />

			{tabs.map((tab) => {
				if (tab.what != current) return null;

				return constructors[tab.what]({ what: tab.what });
			})}
		</TerminalWrapper>
	);
}
