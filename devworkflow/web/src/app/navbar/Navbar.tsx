// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import classNames from "classnames";
import { Link } from "wouter";
import { useData } from "../../datamodel/StackObserver";
import { Carret } from "../../ui/combobox/ComboBox";
import { useCurrentServer } from "../server/ServerPanel";
import { CurrentServer } from "../sidebar/ServerBlock";
import classes from "./navbar.module.css";

export function Navbar() {
	return (
		<div className={classes.fixedNavbar}>
			<Selector />
			<div className={classes.navbarMain}>
				<Searchbox />
			</div>
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

export function Logo() {
	const logoURL = new URL("/icons/logo.svg", import.meta.url);
	return (
		<Link href="/">
			<a className={classes.logo}>
				<div className={classes.foundation}>foundation</div>
				<div className={classes.attribution}>
					<span>by </span>
					<div className={classes.namespaceLabs}>
						<img className={classes.namespaceLabsImg} src={logoURL.toString()}></img>
						<div className={classes.namespaceLabsText}>
							<span>namespace</span>
							<span>labs</span>
						</div>
					</div>
				</div>
			</a>
		</Link>
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
