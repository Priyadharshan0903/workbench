#!/usr/bin/env node
const { spawn } = require('node:child_process');
const os = require('node:os');
const packageInfo = require('../package.json');
const { installedBinary, nativeArguments } = require('./runtime.cjs');

function main() {
  const { executable, root } = installedBinary(packageInfo);
  const child = spawn(executable, nativeArguments(process.argv.slice(2), root), {
    stdio: 'inherit',
    env: process.env,
  });

  const forwarders = new Map();
  for (const signal of ['SIGINT', 'SIGTERM', 'SIGHUP']) {
    const forward = () => {
      if (child.exitCode === null && child.signalCode === null) child.kill(signal);
    };
    forwarders.set(signal, forward);
    process.on(signal, forward);
  }

  function removeForwarders() {
    for (const [signal, forward] of forwarders) process.removeListener(signal, forward);
  }

  child.once('error', (error) => {
    removeForwarders();
    console.error(`Workbench: ${error.message}`);
    process.exitCode = 1;
  });
  child.once('exit', (code, signal) => {
    removeForwarders();
    process.exitCode = code ?? (128 + (os.constants.signals[signal] || 1));
  });
}

try {
  main();
} catch (error) {
  // stdout belongs exclusively to the Go CLI, including its MCP transport.
  console.error(`Workbench: ${error.message}`);
  process.exitCode = 1;
}
