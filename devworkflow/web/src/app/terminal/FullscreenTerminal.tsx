// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import ServerTerminal from "../server/ServerTerminal";

export default function FullscreenTerminal(props: { id: string }) {
  return (
    <ServerTerminal serverId={props.id} what="terminal" wireStdin={true} />
  );
}