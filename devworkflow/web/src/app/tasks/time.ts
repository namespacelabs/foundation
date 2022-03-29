// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import formatDate from "date-fns/format";
import formatDuration from "date-fns/formatDuration";
import differenceInMilliseconds from "date-fns/differenceInMilliseconds";

export function formatTime(t: string) {
  // XXX don't redo work all the time.
  const d = new Date(parseInt(t) / 1000000);

  return formatParsedDateTime(d);
}

export function formatParsedDateTime(d: Date) {
  return formatDate(d, "MMM dd HH:mm:ss.SSS");
}

export function formatParsedTime(d: Date) {
  return formatDate(d, "HH:mm:ss.SSS");
}

export function formatDur(start: string, end: string) {
  const a = new Date(parseInt(start) / 1000000);
  const b = new Date(parseInt(end) / 1000000);
  return formatParsedDur(differenceInMilliseconds(b, a));
}

export function formatParsedDur(diff: number) {
  return formatDuration({ seconds: diff / 1000 });
}