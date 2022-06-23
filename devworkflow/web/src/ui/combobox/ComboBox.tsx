// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import classNames from "classnames";
import React from "react";
import { chevronDownData, makeIcon } from "../../icons";
import classes from "./combobox.module.css";

export default function ComboBox(props: {
	pinned?: boolean;
	compact?: boolean;
	children: React.ReactNode;
}) {
	return (
		<div
			className={classNames(classes.combobox, {
				[classes.pinned]: props.pinned,
				[classes.compact]: props.compact,
			})}>
			<div>{props.children}</div>
			<Carret />
		</div>
	);
}

export function Carret() {
	return <div className={classes.carret}>{makeIcon(chevronDownData)}</div>;
}
