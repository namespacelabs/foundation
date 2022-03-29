// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { useData } from "../../datamodel/StackObserver";
import { Sidebar } from "../../ui/sidebar/Sidebar";
import ServerBlock from "./ServerBlock";

export default function AppSidebar(props: {
  fixed?: boolean;
  footer?: JSX.Element;
}) {
  const data = useData();

  return (
    <Sidebar {...props}>
      {data?.current ? <ServerBlock data={data} /> : null}
    </Sidebar>
  );
}