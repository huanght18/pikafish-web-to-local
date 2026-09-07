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
  const elementsById = new Map();
  const footer = {
    children: [],
    append(element) {
      element.parentElement?.removeChild?.(element);
      this.children.push(element);
      element.parentElement = this;
      elementsById.set(element.id, element);
    },
    removeChild(element) {
      this.children = this.children.filter((child) => child !== element);
      element.parentElement = null;
      elementsById.delete(element.id);
    },
  };
  const document = {
    documentElement: {},
    addEventListener() {},
    createElement(tagName) {
      return {
        tagName: tagName.toUpperCase(),
        id: "",
        textContent: "",
        title: "",
        style: {},
        parentElement: null,
        attributes: {},
        setAttribute(name, value) {
          this.attributes[name] = value;
        },
        remove() {
          this.parentElement?.removeChild(this);
        },
      };
    },
    getElementById(id) {
      return elementsById.get(id) ?? null;
    },
    querySelector(selector) {
      return selector === ".footer" ? footer : null;
    },
  };
  class FakeMutationObserver {
    constructor(callback) {
      this.callback = callback;
    }

    observe() {}
  }

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
    document,
    MutationObserver: FakeMutationObserver,
    WebSocket: FakeWebSocket,
    console: { log() {}, warn() {} },
    setTimeout,
    clearTimeout,
    queueMicrotask,
  };
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox);
  return { window: sandbox.window, sockets, document, footer };
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

async function testConnectionTagTracksLocalService() {
  const harness = createHarness();
  const tag = harness.document.getElementById("pkf-local-connection-tag");
  assert.equal(tag?.textContent, "油猴已加载");

  await patchedEngine(harness);
  const [socket] = harness.sockets;
  assert.equal(tag?.textContent, "油猴已加载 · 连接中");

  socket.open();
  assert.equal(tag?.textContent, "油猴已加载 · 本地已连接");
  assert.equal(tag?.parentElement, harness.footer);

  socket.remoteClose();
  assert.equal(tag?.textContent, "油猴已加载 · 本地未连接");
}

Promise.resolve()
  .then(testQueuesUntilConnectedAndRestoresAfterDisconnect)
  .then(testInitialFailureFallsBackAsOneSession)
  .then(testConnectionTagTracksLocalService)
  .then(() => console.log("pkf-local tests: PASS"))
  .catch((error) => {
    console.error(error);
    process.exitCode = 1;
  });
