// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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
