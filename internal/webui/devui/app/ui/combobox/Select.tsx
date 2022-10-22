// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import ComboBox from "./ComboBox";
import classes from "./combobox.module.css";

export default function Select(props: {
	compact?: boolean;
	items: string[];
	selected?: string;
	onChange?: (value: string) => void;
}) {
	return (
		<ComboBox compact={props.compact}>
			<select
				className={classes.select}
				value={props.selected}
				onChange={(ev) => (props.onChange ? props.onChange(ev.target.value) : null)}>
				{props.items.map((it) => (
					<option key={it} value={it}>
						{it}
					</option>
				))}
			</select>
		</ComboBox>
	);
}
