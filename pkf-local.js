// ==UserScript==
// @name         pkf-local
// @match        https://xiangqiai.com/*
// @run-at       document-start
// @grant        none
// ==/UserScript==

(() => {
  const WS_URL = "ws://localhost:8765";

  // --- WS 管理：一条连接，接收每行输出 ---
  let ws = null;
  function ensureWS(onLine) {
    if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) return ws;

    ws = new WebSocket(WS_URL);
    ws.onopen = () => console.log("[HACK] ws connected:", WS_URL);
    ws.onerror = (e) => console.warn("[HACK] ws error:", e);
    ws.onclose = () => console.warn("[HACK] ws closed");

    ws.onmessage = (ev) => {
      // 你 servepkf.py 是“一行一个 message”，这里就“一行触发一次”
      onLine(String(ev.data ?? ""));
    };
    return ws;
  }

  // --- 把一个 Pikafish 函数“包装”为：sendCommand -> ws，ws输出 -> 原 onReceiveStdout ---
  function wrapPikafish(origPikafish) {
    if (origPikafish && origPikafish.__localWrapped) return origPikafish;

    function patchedPikafish(opts, ...rest) {
      const uiStdout = opts?.onReceiveStdout; // 原网页解析器（你的 onReceiveOutput）
      const uiExit = opts?.onExit;

      // 确保 WS，并把每行喂给原解析器
      ensureWS((line) => {
        try { uiStdout && uiStdout(line); } catch (e) { console.warn("[HACK] uiStdout error", e); }
      });

      // 继续创建原 wasm 引擎（让网页其它逻辑正常）
      const p = origPikafish.call(this, opts, ...rest);

      // 拿到 wasm engine 后，替换它的 sendCommand
      Promise.resolve(p).then((engine) => {
        if (!engine || engine.__sendCommandPatched) return;
        engine.__sendCommandPatched = true;

        const origSend = engine.sendCommand?.bind(engine);
        engine.sendCommand = (cmd) => {
          cmd = String(cmd ?? "").trim();
          console.log("[HACK] < " + cmd);

          const sock = ensureWS((line) => {
            try { uiStdout && uiStdout(line); } catch {}
          });

          if (sock.readyState === WebSocket.OPEN) {
            sock.send(cmd);
          } else {
            console.warn("[HACK] ws not ready, fallback to wasm sendCommand");
            origSend && origSend(cmd);
          }
        };

        // exit 事件仍交给网页
        if (uiExit && typeof uiExit === "function" && typeof engine.onExit === "function") {
          // 有些引擎对象没有 onExit 可挂；这段只是“尽量保持原行为”
          try { engine.onExit = uiExit; } catch {}
        }

        console.log("[HACK] wasm engine sendCommand patched -> local ws");
      });

      return p;
    }

    patchedPikafish.__localWrapped = true;
    patchedPikafish.__orig = origPikafish;
    return patchedPikafish;
  }

  // --- 关键：用 defineProperty 拦截 window.Pikafish 的“后续赋值”，确保永不被覆盖 ---
  let _pikafishValue = undefined;

  Object.defineProperty(window, "Pikafish", {
    configurable: true,
    enumerable: true,
    get() {
      return _pikafishValue;
    },
    set(v) {
      // 页面任何时候给 window.Pikafish 赋值，我们都包一层再放进去
      _pikafishValue = (typeof v === "function") ? wrapPikafish(v) : v;
      console.log("[HACK] window.Pikafish set -> wrapped:", _pikafishValue);
    },
  });

  // 如果脚本注入时 Pikafish 已经存在（极少数情况下），也包一下
  try {
    if (typeof window.Pikafish === "function") {
      window.Pikafish = window.Pikafish; // 触发 setter，自动 wrap
    }
  } catch {}

  console.log("[HACK] installed (document-start).");
})();
