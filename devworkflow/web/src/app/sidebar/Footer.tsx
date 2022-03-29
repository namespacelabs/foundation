// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { makeIcon, historyData, cmdData, monitorData } from "../../icons";
import { Item } from "../../ui/sidebar/Sidebar";
import { useTasksRoute } from "../tasks/routing";
import { useCommandRoute } from "../command/routing";
import { useBuildRoute } from "../build/routing";

export function FooterItems() {
  return (
    <>
      <Build />
      <Command />
      <Tasks />
    </>
  );
}

function Build() {
  const [matches, _] = useBuildRoute();

  return (
    <Item href="/build" icon={makeIcon(monitorData)} active={matches}>
      Build
    </Item>
  );
}

function Command() {
  const [matches, _] = useCommandRoute();

  return (
    <Item href="/command" icon={makeIcon(cmdData)} active={matches}>
      Console
    </Item>
  );
}

function Tasks() {
  const [matches, _] = useTasksRoute();

  return (
    <Item href="/tasks" icon={makeIcon(historyData)} active={matches}>
      Tasks
    </Item>
  );
}