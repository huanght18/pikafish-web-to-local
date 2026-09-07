// ==UserScript==
// @name         pkf-local
// @match        https://xiangqiai.com/*
// @run-at       document-start
// @grant        none
// ==/UserScript==

(() => {
  "use strict";

  const WS_URL = "ws://localhost:8765";
  const CONNECT_TIMEOUT_MS = 1500;

  // 每个网页引擎实例拥有自己的 WS，和服务端“一连接一进程”的模型对应。
  // 初次连接期间先排队，不把同一会话拆到本地引擎和 WASM 两边。
  function createBridge(onLine, onFallback) {
    let socket = null;
    let state = "connecting"; // connecting | local | wasm | closed
    let pending = [];
    let timer = null;

    const switchToWasm = (reason, replayPending) => {
      if (state === "wasm" || state === "closed") return;
      const queued = replayPending ? pending.splice(0) : null;
      state = "wasm";
      clearTimeout(timer);
      console.warn(`[HACK] ${reason}; using webpage WASM for this engine session`);
      try { socket?.close(); } catch {}
      onFallback(queued);
    };

    try {
      socket = new WebSocket(WS_URL);
    } catch (error) {
      console.warn("[HACK] cannot create websocket:", error);
      state = "wasm";
      queueMicrotask(() => onFallback([]));
    }

    if (socket) {
      timer = setTimeout(
        () => switchToWasm("local service connection timed out", true),
        CONNECT_TIMEOUT_MS,
      );

      socket.onopen = () => {
        if (state !== "connecting") {
          socket.close();
          return;
        }
        state = "local";
        clearTimeout(timer);
        console.log("[HACK] ws connected:", WS_URL);
        const queued = pending.splice(0);
        try {
          for (const command of queued) socket.send(command);
        } catch (error) {
          console.warn("[HACK] flushing queued commands failed:", error);
          switchToWasm("local connection failed", false);
        }
      };

      socket.onerror = (error) => console.warn("[HACK] ws error:", error);
      socket.onclose = () => {
        if (state === "connecting") {
          switchToWasm("local service unavailable", true);
        } else if (state === "local") {
          switchToWasm("local service disconnected", false);
        }
      };
      socket.onmessage = (event) => {
        if (state !== "local") return;
        onLine(String(event.data ?? ""));
      };
    }

    return {
      send(command) {
        if (state === "connecting") {
          pending.push(command);
          return true;
        }
        if (state !== "local" || socket?.readyState !== WebSocket.OPEN) {
          if (state === "local") switchToWasm("local connection is no longer open", false);
          return false;
        }
        try {
          socket.send(command);
          return true;
        } catch (error) {
          console.warn("[HACK] ws send failed:", error);
          switchToWasm("local send failed", false);
          return false;
        }
      },
      close() {
        state = "closed";
        pending = [];
        clearTimeout(timer);
        try { socket?.close(); } catch {}
      },
    };
  }

  // 保存足以在本地连接中断后恢复 WASM 状态的命令，而不是重放历史搜索。
  function createSessionState() {
    let sawUci = false;
    let newGame = false;
    let position = null;
    let activeGo = null;
    const options = new Map();

    return {
      observe(command) {
        const lower = command.toLowerCase();
        if (lower === "uci") sawUci = true;
        else if (lower === "ucinewgame") {
          newGame = true;
          position = null;
          activeGo = null;
        } else if (lower.startsWith("setoption ")) {
          const match = command.match(/^setoption\s+name\s+(.+?)(?:\s+value\s+.*)?$/i);
          options.set((match?.[1] ?? command).toLowerCase(), command);
        } else if (lower.startsWith("position ") || lower.startsWith("fen ")) {
          position = command;
          activeGo = null;
        } else if (lower === "stop" || lower === "quit") {
          activeGo = null;
        } else if (lower === "go" || lower.startsWith("go ")) {
          activeGo = command;
        }
      },
      analysisFinished() {
        activeGo = null;
      },
      replay(send) {
        if (sawUci) send("uci");
        for (const command of options.values()) send(command);
        if (newGame) send("ucinewgame");
        if (position) send(position);
        if (activeGo) send(activeGo);
      },
    };
  }

  function wrapPikafish(origPikafish) {
    if (origPikafish && origPikafish.__localWrapped) return origPikafish;

    function patchedPikafish(opts, ...rest) {
      const uiStdout = opts?.onReceiveStdout;
      const result = origPikafish.call(this, opts, ...rest);

      Promise.resolve(result).then((engine) => {
        if (!engine || engine.__sendCommandPatched) return;
        engine.__sendCommandPatched = true;

        const origSend = engine.sendCommand?.bind(engine);
        const session = createSessionState();
        let backend = "pending";

        const sendToWasm = (command) => {
          try { origSend?.(command); } catch (error) {
            console.warn("[HACK] wasm sendCommand failed:", error);
          }
        };

        const bridge = createBridge(
          (line) => {
            if (line.startsWith("bestmove")) session.analysisFinished();
            try { uiStdout?.(line); } catch (error) {
              console.warn("[HACK] uiStdout error:", error);
            }
          },
          (queued) => {
            if (backend === "wasm") return;
            backend = "wasm";
            if (queued) {
              // 本地端从未接收过命令，按原顺序交给 WASM。
              for (const command of queued) sendToWasm(command);
            } else {
              // 本地会话中途断开：恢复必要状态，并继续未完成的搜索。
              session.replay(sendToWasm);
            }
          },
        );

        engine.sendCommand = (value) => {
          const command = String(value ?? "").trim();
          if (!command) return;
          console.log("[HACK] < " + command);

          if (backend === "wasm") {
            sendToWasm(command);
            return;
          }

          if (bridge.send(command)) {
            session.observe(command);
            backend = "local";
          } else {
            // bridge 会先恢复 WASM 状态；这里只补发尚未成功送达的当前命令。
            backend = "wasm";
            sendToWasm(command);
          }
        };

        if (typeof engine.terminate === "function") {
          const origTerminate = engine.terminate.bind(engine);
          engine.terminate = (...args) => {
            bridge.close();
            return origTerminate(...args);
          };
        }

        console.log("[HACK] wasm engine sendCommand patched -> local ws");
      }).catch((error) => console.warn("[HACK] engine initialization failed:", error));

      return result;
    }

    patchedPikafish.__localWrapped = true;
    patchedPikafish.__orig = origPikafish;
    return patchedPikafish;
  }

  // 拦截后续赋值，避免目标网站加载 Pikafish 时覆盖包装。
  let pikafishValue;
  Object.defineProperty(window, "Pikafish", {
    configurable: true,
    enumerable: true,
    get: () => pikafishValue,
    set(value) {
      pikafishValue = typeof value === "function" ? wrapPikafish(value) : value;
      console.log("[HACK] window.Pikafish set -> wrapped:", pikafishValue);
    },
  });

  console.log("[HACK] installed (document-start).");
})();
