// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import classNames from "classnames";
import { ReactNode } from "react";
import { Link } from "wouter";
import classes from "./sidebar.module.css";

export function Sidebar(props: {
  fixed?: boolean;
  children: ReactNode;
  footer?: ReactNode;
}) {
  return (
    <div
      className={classNames(classes.sidebar, { [classes.fixed]: props.fixed })}
    >
      <div className={classes.sidebarMain}>{props.children}</div>
      {props.footer ? (
        <div className={classes.sidebarFooter}>{props.footer}</div>
      ) : null}
    </div>
  );
}

export function Header(props: { children: ReactNode }) {
  return (
    <div className={classNames(classes.sidebarHeader)}>{props.children}</div>
  );
}

export function Block(props: { children: ReactNode }) {
  return <div className={classes.itemBlock}>{props.children}</div>;
}

export function ItemRow(props: { children: ReactNode }) {
  return <div className={classes.itemRow}>{props.children}</div>;
}

export function ItemSpacer() {
  return <div style={{ flex: 1 }} />;
}

export function Item(props: {
  icon?: ReactNode;
  href?: string;
  children: ReactNode;
  rightSide?: ReactNode;
  active?: boolean;
}) {
  let body = (
    <a
      className={classNames(classes.item, {
        [classes.itemActive]: props.active,
      })}
    >
      <div className={classes.icon}>{props.icon}</div>
      <div className={classes.itemContent}>{props.children}</div>
      {props.rightSide ? (
        <div className={classes.itemRight}>{props.rightSide}</div>
      ) : null}
    </a>
  );

  return props.href ? <Link href={props.href}>{body}</Link> : body;
}

export function Collapsable(props: {
  icon: JSX.Element;
  title: string;
  children: ReactNode;
}) {
  return (
    <Block>
      <Item
        icon={props.icon}
        rightSide={
          <span className={classes.carretDown}>
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16">
              <path d="M14.15 4.492H1.85l-.354.854 6.15 6.15h.707l6.15-6.15-.353-.854z"></path>
            </svg>
          </span>
        }
      >
        <span>{props.title}</span>
      </Item>
      {props.children}
    </Block>
  );
}