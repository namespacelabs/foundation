#!/usr/bin/env node
// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

const { spawn } = require("child_process");
const { install, update, getBinaryPath, isInstalled } = require("../lib/install");

async function main() {
  const args = process.argv.slice(2);

  if (args[0] === "update") {
    try {
      const result = await update();
      if (result.alreadyLatest) {
        console.log(`nsc is already at the latest version (${result.version}).`);
      } else {
        console.log("nsc has been updated to the latest version.");
      }
      process.exit(0);
    } catch (err) {
      console.error("Failed to update nsc:", err.message);
      process.exit(1);
    }
  }

  try {
    if (!isInstalled()) {
      await install();
    }
  } catch (err) {
    console.error("Failed to install nsc:", err.message);
    process.exit(1);
  }

  const binaryPath = getBinaryPath();
  const child = spawn(binaryPath, args, {
    stdio: "inherit",
    env: {
      ...process.env,
      NSBOOT_VERSION: JSON.stringify({ source: "@namespacelabs/cli" }),
      NS_DO_NOT_UPDATE: "1",
    },
  });

  child.on("error", (err) => {
    console.error("Failed to run nsc:", err.message);
    process.exit(1);
  });

  child.on("close", (code) => {
    process.exit(code || 0);
  });
}

main();
