// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import React from "react";
import { Link } from "wouter";
import classes from "./chrome.module.css";
import classNames from "classnames";

type Tab = {
	what: string;
	label: string;
	hdrRef?: React.MutableRefObject<any>;
};

export default function TerminalTabs(props: {
	prepend?: JSX.Element;
	tabs: Tab[];
	makeHref?: (what: string) => string;
	current: string;
	children?: React.ReactNode;
}) {
	// XXX this is not great as we need to know the full route. Where's my encapsulation.
	return (
		<div className={classes.panelHeader}>
			{props.prepend}
			{props.tabs.map((tab) => {
				let body = (
					<a
						key={tab.what}
						ref={tab.hdrRef}
						className={classNames(classes.panelHeaderItem, {
							[classes.active]: props.current == tab.what,
						})}>
						<span>{tab.label}</span>
					</a>
				);

				if (props.makeHref) {
					return (
						<Link href={props.makeHref(tab.what)} key={tab.what}>
							{body}
						</Link>
					);
				}

				return body;
			})}
			{props.children ? <div className={classes.panelHeaderRight}>{props.children}</div> : null}
		</div>
	);
}
