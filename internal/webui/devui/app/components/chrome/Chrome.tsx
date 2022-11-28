// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import React from "react";
import { Header } from "../header/Header";
import classes from "./chrome.module.css";

export function Chrome(props: {
	children: React.ReactNode;
	headerLabel: React.ReactNode;
	footer?: React.ReactNode;
}) {
	return (
		<div className={classes.chrome}>
			<Header label={props.headerLabel} />
			<main className={classes.main}>{props.children}</main>
			{props.footer}
		</div>
	);
}

export function Navbar(props: { children?: React.ReactNode }) {
	return <div className={classes.navbar}>{props.children}</div>;
}

export function Content(props: { children?: React.ReactNode }) {
	return <div className={classes.content}>{props.children}</div>;
}
