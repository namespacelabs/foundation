// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { useTasks } from "../../datamodel/TasksObserver";
import { OutputSocket } from "../../devworkflow/output";
import Panel from "../../ui/panel/Panel";
import TerminalTabs from "../../ui/termchrome/TerminalTabs";
import { StreamOutput } from "../../ui/terminal/StreamOutput";
import BuildKitLog from "../../ui/buildkit/BuildkitLog";
import classes from "./tasks.module.css";
import { useLocation } from "wouter";

export default function Task(props: { id: string; what?: string }) {
	let [_, setLocation] = useLocation();
	let tasks = useTasks();

	// XXX maybe an index.
	let filtered = tasks.filter((t) => t.id == props.id);
	if (!filtered.length) {
		return <div>404</div>;
	}

	let t = filtered[0];
	let tabs = t.output?.map((output) => ({
		what: output.name,
		label: output.name,
	}));

	if (!tabs) {
		return <div>Task produced no outputs.</div>;
	}

	let current = props.what || tabs[0].what;
	let nonBuildkit = (t.output || [])
		.filter((output) => output.content_type !== "application/json+fn.buildkit")
		.map((output) => output.name);
	let buildkit = (t.output || [])
		.filter((output) => output.content_type === "application/json+fn.buildkit")
		.map((output) => output.name);

	return (
		<Panel key={t.id}>
			<TerminalTabs
				prepend={
					<button
						onClick={() => void setLocation("/tasks", { replace: true })}
						className={classes.backButton}>
						<div>
							<svg
								xmlns="http://www.w3.org/2000/svg"
								x="0px"
								y="0px"
								width="48"
								height="48"
								viewBox="0 0 226 226"
								style={{ fill: "#000000" }}>
								<g
									fill="none"
									fillRule="nonzero"
									stroke="none"
									strokeWidth="1"
									strokeLinecap="butt"
									strokeLinejoin="miter"
									strokeMiterlimit="10"
									strokeDasharray=""
									strokeDashoffset="0"
									fontFamily="none"
									fontWeight="none"
									fontSize="none"
									textAnchor="none"
									style={{ mixBlendMode: "normal" }}>
									<path d="M0,226v-226h226v226z" fill="none"></path>
									<g fill="#ffffff">
										<path d="M181.55804,42.08779l-30.60417,-30.60417c-2.75908,-2.75908 -7.22729,-2.75908 -9.98637,0l-96.52083,96.52083c-2.75908,2.75908 -2.75908,7.22729 0,9.98637l96.52083,96.52083c1.37483,1.38425 3.18283,2.07167 4.99083,2.07167c1.808,0 3.616,-0.68742 4.99554,-2.06696l30.60417,-30.60417c2.75908,-2.75908 2.75908,-7.22729 0,-9.98638l-60.92583,-60.92583l60.92113,-60.92112c2.75908,-2.75908 2.75908,-7.232 0.00471,-9.99108z"></path>
									</g>
								</g>
							</svg>
							<span>Tasks</span>
						</div>
					</button>
				}
				current={current}
				tabs={tabs}
				makeHref={(what) => `/tasks/${props.id}/${what}`}
			/>
			{nonBuildkit.indexOf(current) >= 0 ? (
				<StreamLog id={props.id} what={current} key={`${props.id}/${current}`} />
			) : null}
			{buildkit.indexOf(current) >= 0 ? (
				<BuildKitLog apiUrl={`task/${props.id}/output/${current}`} key={`${props.id}/${current}`} />
			) : null}
		</Panel>
	);
}

function StreamLog(props: { id: string; what: string }) {
	return (
		<StreamOutput
			makeSocket={() =>
				new OutputSocket({
					endpoint: `task/${props.id}/output/${props.what}`,
				})
			}
		/>
	);
}
