// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import classNames from "classnames";
import { Link } from "wouter";
import { useData } from "../../datamodel/StackObserver";
import { Carret } from "../../ui/combobox/ComboBox";
import { LogoIcon } from "../logo/Logo";
import { useCurrentServer } from "../server/ServerPanel";
import { CurrentServer } from "../sidebar/ServerBlock";
import classes from "./navbar.module.css";

export function Navbar() {
	return (
		<div className={classes.fixedNavbar}>
			<Link href="/">
				<a>
					<LogoIcon filled />
				</a>
			</Link>
			<div className={classes.navbarMain}>Development UI</div>
		</div>
	);
}

function Selector() {
	let data = useData();
	let current = useCurrentServer();

	return (
		<div
			className={classNames(classes.selector, {
				[classes.active]: !!current?.server,
			})}>
			<div>{data ? <CurrentServer data={data} /> : null}</div>
			<div>
				<Carret />
			</div>
		</div>
	);
}

function Searchbox() {
	return (
		<div className={classes.searchBox}>
			<input placeholder="Search server graph, logs, etc." />
			<div className={classes.searchHint}>⌘K</div>
		</div>
	);
}
