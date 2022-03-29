// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import classNames from "classnames";
import { useState } from "react";
import classes from "./sidebar.module.css";

type Tab = {
  id: string;
  label: string;
  render: (args: { id: string }) => JSX.Element;
};

export default function Tabs(props: { tabs: Tab[]; rightSide?: JSX.Element }) {
  let [current, setCurrent] = useState(null);

  let currentTab = props.tabs.filter((t) => !current || t.id === current);
  if (!currentTab.length) {
    return null;
  }

  let tab = currentTab[0];

  return (
    <div className={classes.tabs}>
      <div className={classes.tabHeader}>
        <div className={classes.tabLeft}>
          {props.tabs.map((t) => (
            <div
              key={t.id}
              className={classNames(classes.tabItem, {
                [classes.activeTab]: t.id === tab.id,
              })}
            >
              {t.label}
            </div>
          ))}
        </div>
        {props.rightSide}
      </div>
      <div className={classes.tabBody}>{tab.render(tab)}</div>
    </div>
  );
}