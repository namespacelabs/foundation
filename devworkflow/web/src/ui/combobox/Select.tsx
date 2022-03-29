// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

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
        onChange={(ev) =>
          props.onChange ? props.onChange(ev.target.value) : null
        }
      >
        {props.items.map((it) => (
          <option key={it} value={it}>
            {it}
          </option>
        ))}
      </select>
    </ComboBox>
  );
}