// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import React, { useRef } from "react";
import { useData } from "../../datamodel/StackObserver";
import { useServerRoute } from "./routing";
import TerminalTabs from "../../ui/termchrome/TerminalTabs";
import ServerTerminal from "./ServerTerminal";
import { StackEntryType } from "../../datamodel/Schema";
import classes from "./server.module.css";

export function useCurrentServer() {
  let [match, params] = useServerRoute();
  let data = useData();

  if (match && params?.id) {
    let server = data?.stack?.entry?.filter((s) => s.server.id === params?.id);
    if (server?.length && data?.stack) {
      return {
        env: data.env,
        stack: data.stack,
        server: server[0],
        current: params.tab || "logs",
      };
    }
  }

  return null;
}

export function ServerTabs(props: { server: StackEntryType; current: string }) {
  let tabs = [
    { what: "logs", label: "Output", hdrRef: useRef(null) },
    { what: "terminal", label: "Terminal", hdrRef: useRef(null) },
  ];
  let { id } = props.server.server;

  type TabProps = { hdrRef: React.MutableRefObject<any>; what: string };

  let constructors: { [key: string]: (tabProps: TabProps) => JSX.Element } = {
    logs: (tabProps: TabProps) => (
      <ServerTerminal key={`logs/${id}`} serverId={id} {...tabProps} />
    ),
    terminal: (tabProps: TabProps) => (
      <ServerTerminal
        key={`terminal/${id}`}
        serverId={id}
        wireStdin={true}
        {...tabProps}
      />
    ),
  };

  return (
    <>
      <TerminalTabs
        tabs={tabs}
        makeHref={(what: string) => `/server/${id}/${what}`}
        current={props.current}
      ></TerminalTabs>
      <div className={classes.terminalWrapper}>
        {tabs.map((tab) => {
          if (tab.what != props.current) return null;

          return constructors[tab.what]({ hdrRef: tab.hdrRef, what: tab.what });
        })}
      </div>
    </>
  );
}