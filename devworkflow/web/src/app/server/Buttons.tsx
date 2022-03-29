// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { useContext } from "react";
import { useLocation } from "wouter";
import Button from "../../ui/button/Button";
import { WSContext } from "../../datamodel/StackObserver";

export function RebuildButton() {
  let ws = useContext(WSContext);
  let [_, setLocation] = useLocation();

  return (
    <Button
      onClick={() => {
        if (ws) {
          ws.send({ reloadWorkspace: true });
          setLocation(`/server/7hzne001dff2rpdxav703bwqwc/command`);
        }
      }}
    >
      Rebuild 🐞
    </Button>
  );
}

export function NewTerminalButton(props: { id: string }) {
  return (
    <Button
      onClick={() => {
        window.open(`/terminal/${props.id}`, "_blank");
      }}
    >
      New Terminal
    </Button>
  );
}