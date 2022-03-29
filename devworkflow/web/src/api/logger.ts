// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

export const InfoLevel = 1;
export const DebugLevel = 2;

export class Logger {
  private readonly what: string;
  private level: number;

  constructor(what: string, level: number = InfoLevel) {
    this.what = what;
    this.level = level;
  }

  info(...args: any[]) {
    if (InfoLevel <= this.level) {
      console.log(`info[${this.what}]`, ...args);
    }
  }

  debug(...args: any[]) {
    if (DebugLevel <= this.level) {
      console.log(`debug[${this.what}]`, ...args);
    }
  }

  error(...args: any[]) {
    console.error(`error[${this.what}]`, ...args);
  }
}