// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { useTasksRoute } from "./routing";
import Task from "./Task";
import TaskList from "./TaskList";

export default function TasksPanel() {
  const [match, params] = useTasksRoute();

  if (match) {
    if (params?.id) {
      return <Task id={params.id} what={params.what} />;
    } else {
      return <TaskList />;
    }
  }

  return null;
}