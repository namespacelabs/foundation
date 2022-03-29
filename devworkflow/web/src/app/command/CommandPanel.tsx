// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import TerminalTabs from "../../ui/termchrome/TerminalTabs";
import StreamSocket from "../server/StreamSocket";
import { useCommandRoute } from "./routing";

export default function CommandPanel() {
  const [match, _] = useCommandRoute();

  if (!match) {
    return null;
  }

  let tabs = [{ what: "command", label: "Output" }];

  type TabProps = { what: string };

  let constructors: { [key: string]: (tabProps: TabProps) => JSX.Element } = {
    command: (tabProps: TabProps) => (
      <StreamSocket key="command/output" {...tabProps} />
    ),
  };

  let current = tabs[0].what;

  return (
    <>
      <TerminalTabs
        tabs={tabs}
        makeHref={(what: string) => "/command"}
        current={current}
      />

      {tabs.map((tab) => {
        if (tab.what != current) return null;

        return constructors[tab.what]({ what: tab.what });
      })}
    </>
  );
}