// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { useContext } from "react";
import { DataType } from "../../datamodel/Schema";
import { WSContext } from "../../datamodel/StackObserver";
import { useLocation } from "wouter";
import Select from "../../ui/combobox/Select";

export default function EnvList(props: { data: DataType }) {
  let ws = useContext(WSContext);
  let [_, setLocation] = useLocation();
  let { abs_root: absRoot, current, env, workspace } = props.data;

  return (
    <Select
      compact={true}
      items={workspace.env.map((env) => env.name)}
      selected={env.name}
      onChange={(env) => {
        if (ws) {
          ws.send({
            setWorkspace: {
              absRoot,
              packageName: current.server.package_name,
              envName: env,
            },
          });
          setLocation("/command");
        }
      }}
    />
  );
}