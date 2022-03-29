// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { useEffect, useState } from "react";
import { Socket } from "../../api/websocket";
import Panel from "../panel/Panel";
import classes from "./buildkit.module.css";
import classNames from "classnames";
import { renderParts } from "./Segment";
import { WireEvent, BuildInvocation } from "./BuildEvent";
import { formatParsedDur, formatParsedTime } from "../../app/tasks/time";

type BuildInvocationMap = { [id: string]: BuildInvocation };

export default function BuildKitLog(props: { apiUrl: string }) {
  let [map, setMap] = useState<{
    serial: number;
    invocations: BuildInvocationMap;
    keys: string[];
  }>({ serial: 0, invocations: {}, keys: [] });

  useEffect(() => {
    const conn = new BuildKitSocket(props.apiUrl);
    conn.ensureConnected();

    const release = conn.observe((invocations, keys) =>
      setMap((p) => ({ serial: p.serial + 1, invocations, keys }))
    );

    return () => {
      release();
      conn.close();
    };
  }, []);

  return (
    <Panel>
      <div className={classes.buildkitLog}>
        {map.keys
          .map((key) => map.invocations[key])
          .map((invocation) => (
            <div key={invocation.id} className={classes.invocation}>
              {invocation.sorted().map((ev) => (
                <div key={ev.digest}>
                  <div
                    className={classNames(classes.line, {
                      [classes.was]: ev.cached,
                    })}
                  >
                    <div className={classes.time}>{ev.startedStr}</div>
                    <div className={classes.cached}>
                      <div />
                    </div>
                    <div>{renderParts(ev.parts)}</div>
                    <div className={classes.duration}>{ev.durationStr}</div>
                  </div>
                  {ev.statuses.map((s) => (
                    <div className={classes.line}>
                      <div className={classes.time}>{s.startedStr}</div>
                      <div className={classes.cached} />
                      <div>&#8627; {renderParts(s.parts)}</div>
                      <div className={classes.duration}>{s.durationStr}</div>
                    </div>
                  ))}
                </div>
              ))}
              {invocation.started && invocation.completed ? (
                <div className={classes.line}>
                  <div className={classes.time}>
                    {formatParsedTime(new Date(invocation.completed))}
                  </div>
                  <div className={classes.cached} />
                  <div>
                    Done, took{" "}
                    {formatParsedDur(invocation.completed - invocation.started)}
                    .
                  </div>
                  <div className={classes.duration} />
                </div>
              ) : null}
            </div>
          ))}
      </div>
    </Panel>
  );
}

class BuildKitSocket extends Socket {
  private observers: ((
    invocations: BuildInvocationMap,
    keys: string[]
  ) => void)[] = [];
  private invocations: BuildInvocationMap = {};
  private ids: string[] = [];
  private buf: string = "";

  constructor(apiUrl: string) {
    super({ kind: apiUrl, apiUrl: apiUrl, autoReconnect: false });
  }

  protected override async onMessage(data: any) {
    const text: string = await data.text();

    this.buf += text;

    while (true) {
      let nl = this.buf.indexOf("\n");
      if (nl < 0) {
        break;
      }

      let msgText = this.buf.substring(0, nl);
      this.buf = this.buf.substring(nl + 1);

      let msg: WireEvent = JSON.parse(msgText);
      this.logger.debug("parsed", msg);

      if (!this.invocations[msg.s]) {
        if (!msg.started) {
          continue;
        }

        let started = Date.parse(msg.started);

        this.invocations[msg.s] = new BuildInvocation(msg.s, started);
        this.ids.push(msg.s);
      }

      this.invocations[msg.s].parse(msg);
    }

    for (let id of this.ids) {
      this.invocations[id].tidy();
    }

    this.ids.sort((a, b) => {
      return this.invocations[a].started - this.invocations[b].started;
    });

    this.observers.forEach((f) => f(this.invocations, this.ids));
  }

  observe(observer: (messages: BuildInvocationMap, keys: string[]) => void) {
    this.observers.push(observer);

    observer(this.invocations, this.ids);

    return () => {
      this.observers = this.observers.filter((v) => v != observer);
    };
  }
}