"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const source = fs.readFileSync(
  path.join(__dirname, "..", "tampermonkey", "pkf-local.js"),
  "utf8",
);

function createHarness() {
  const sockets = [];
  class FakeWebSocket {
    static CONNECTING = 0;
    static OPEN = 1;
    static CLOSING = 2;
    static CLOSED = 3;

    constructor(url) {
      this.url = url;
      this.readyState = FakeWebSocket.CONNECTING;
      this.sent = [];
      sockets.push(this);
    }

    open() {
      this.readyState = FakeWebSocket.OPEN;
      this.onopen?.();
    }

    send(command) {
      if (this.readyState !== FakeWebSocket.OPEN) throw new Error("not open");
      this.sent.push(command);
    }

    receive(line) {
      this.onmessage?.({ data: line });
    }

    remoteClose() {
      this.readyState = FakeWebSocket.CLOSED;
      this.onclose?.();
    }

    close() {
      this.readyState = FakeWebSocket.CLOSED;
      this.onclose?.();
    }
  }

  const sandbox = {
    window: {},
    WebSocket: FakeWebSocket,
    console: { log() {}, warn() {} },
    setTimeout,
    clearTimeout,
    queueMicrotask,
  };
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox);
  return { window: sandbox.window, sockets };
}

async function patchedEngine(harness) {
  const wasmCommands = [];
  const output = [];
  const engine = {
    sendCommand(command) {
      wasmCommands.push(command);
    },
    terminate() {},
  };
  harness.window.Pikafish = () => Promise.resolve(engine);
  await harness.window.Pikafish({
    onReceiveStdout(line) {
      output.push(line);
    },
  });
  await Promise.resolve();
  return { engine, wasmCommands, output };
}

async function testQueuesUntilConnectedAndRestoresAfterDisconnect() {
  const harness = createHarness();
  const { engine, wasmCommands, output } = await patchedEngine(harness);
  const socket = harness.sockets[0];

  engine.sendCommand("uci");
  engine.sendCommand("fen test-position");
  assert.deepEqual(wasmCommands, []);
  assert.deepEqual(socket.sent, []);

  socket.open();
  assert.deepEqual(socket.sent, ["uci", "fen test-position"]);

  socket.receive("uciok");
  assert.deepEqual(output, ["uciok"]);

  engine.sendCommand("go depth 8");
  socket.remoteClose();
  assert.deepEqual(wasmCommands, ["uci", "fen test-position", "go depth 8"]);

  engine.sendCommand("stop");
  assert.deepEqual(wasmCommands, ["uci", "fen test-position", "go depth 8", "stop"]);
}

async function testInitialFailureFallsBackAsOneSession() {
  const harness = createHarness();
  const { engine, wasmCommands } = await patchedEngine(harness);
  const socket = harness.sockets[0];

  engine.sendCommand("uci");
  engine.sendCommand("setoption name Threads value 2");
  socket.remoteClose();

  assert.deepEqual(wasmCommands, ["uci", "setoption name Threads value 2"]);
  engine.sendCommand("isready");
  assert.deepEqual(wasmCommands, [
    "uci",
    "setoption name Threads value 2",
    "isready",
  ]);
}

Promise.resolve()
  .then(testQueuesUntilConnectedAndRestoresAfterDisconnect)
  .then(testInitialFailureFallsBackAsOneSession)
  .then(() => console.log("pkf-local tests: PASS"))
  .catch((error) => {
    console.error(error);
    process.exitCode = 1;
  });
