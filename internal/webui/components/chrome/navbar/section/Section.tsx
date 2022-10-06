// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import React from "react";
import classes from "./section.module.css";

export function Section(props: { children?: React.ReactNode; label: string }) {
	return (
		<div className={classes.section}>
			<div className={classes.sectionHeader}>
				<span>{props.label}</span>
			</div>
			<div>{props.children}</div>
		</div>
	);
}
