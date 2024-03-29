// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import React from "react";
import { Link } from "wouter";
import { LogoIcon } from "../logo/Logo";
import classes from "./header.module.css";

export function Header(props: { label: React.ReactNode }) {
	return (
		<div className={classes.header}>
			<Link href="/">
				<a>
					<LogoIcon filled />
				</a>
			</Link>
			<div className={classes.headerContent}>{props.label}</div>
		</div>
	);
}
