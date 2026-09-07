"""Pikafish 本地 WebSocket 桥接服务（Python 版）。
pikafish-web-to-local v2

每个 WebSocket 连接对应一个独立的 Pikafish 子进程。浏览器按 UCI 文本行
发送命令，服务端将引擎 stdout 逐行回传。
"""

import asyncio
import contextlib
import copy
import json
import os
import re
import subprocess
import sys

import websockets


CONFIG_FILENAME = "config.json"
LOG_DIRNAME = "logs"
DEFAULTS = {
    "host": "localhost",
    "port": 8765,
    "engine_path": r"D:\Personal\ChineseChess\pikafish-20260131\pikafish-bmi2.exe",
    "hash_mb": 512,
    "drain_banner": True,
    "allowed_origins": ["https://xiangqiai.com"],
    "max_connections": 4,
    "log": {
        "console": True,
        "file": False,
        "file_path": "pkf-local-python.log",
    },
}

HOST = DEFAULTS["host"]
PORT = DEFAULTS["port"]
ENGINE_PATH = DEFAULTS["engine_path"]
HASH = DEFAULTS["hash_mb"]
DRAIN_BANNER = DEFAULTS["drain_banner"]
ALLOWED_ORIGINS = DEFAULTS["allowed_origins"]
MAX_CONNECTIONS = DEFAULTS["max_connections"]
LOG_CONSOLE = DEFAULTS["log"]["console"]
LOG_FILE = DEFAULTS["log"]["file"]
LOG_FILE_PATH = DEFAULTS["log"]["file_path"]

HASH_COMMAND_RE = re.compile(
    r"^setoption\s+name\s+hash\s+value(?:\s+.*)?$", re.IGNORECASE
)

_log_file_handle = None
_active_connections = 0


def script_dir():
    return os.path.dirname(os.path.abspath(__file__))


def load_config():
    """读取配置，返回 (config, path, created)。"""
    config_path = os.path.join(script_dir(), CONFIG_FILENAME)
    if not os.path.exists(config_path):
        with open(config_path, "w", encoding="utf-8") as file:
            json.dump(DEFAULTS, file, indent=2, ensure_ascii=False)
            file.write("\n")
        return copy.deepcopy(DEFAULTS), config_path, True

    with open(config_path, "r", encoding="utf-8") as file:
        user_cfg = json.load(file)
    if not isinstance(user_cfg, dict):
        raise ValueError("config root must be a JSON object")

    cfg = copy.deepcopy(DEFAULTS)
    for key, value in user_cfg.items():
        if key in cfg and key != "log":
            cfg[key] = value
    if "log" in user_cfg:
        if not isinstance(user_cfg["log"], dict):
            raise ValueError("log must be a JSON object")
        cfg["log"].update(user_cfg["log"])
    return cfg, config_path, False


def validate_config(cfg):
    """在启动监听前给出清晰的配置错误。"""
    if not isinstance(cfg["host"], str) or not cfg["host"].strip():
        raise ValueError("host must be a non-empty string")
    if cfg["host"].strip().lower() not in {"localhost", "127.0.0.1", "::1"}:
        raise ValueError("host must be a loopback address (localhost, 127.0.0.1, or ::1)")
    if not isinstance(cfg["port"], int) or isinstance(cfg["port"], bool) or not 1 <= cfg["port"] <= 65535:
        raise ValueError("port must be an integer between 1 and 65535")
    if not isinstance(cfg["hash_mb"], int) or isinstance(cfg["hash_mb"], bool) or cfg["hash_mb"] <= 0:
        raise ValueError("hash_mb must be a positive integer")
    if not isinstance(cfg["drain_banner"], bool):
        raise ValueError("drain_banner must be a boolean")
    if (
        not isinstance(cfg["max_connections"], int)
        or isinstance(cfg["max_connections"], bool)
        or not 1 <= cfg["max_connections"] <= 32
    ):
        raise ValueError("max_connections must be an integer between 1 and 32")
    origins = cfg["allowed_origins"]
    if not isinstance(origins, list) or not origins or not all(
        isinstance(origin, str) and origin.strip() for origin in origins
    ):
        raise ValueError("allowed_origins must be a non-empty string array")

    engine_path = cfg["engine_path"]
    if not isinstance(engine_path, str) or not os.path.isabs(engine_path):
        raise ValueError("engine_path must be an absolute path")
    if not os.path.isfile(engine_path):
        raise ValueError(f"engine_path does not exist: {engine_path}")

    log_cfg = cfg["log"]
    if not isinstance(log_cfg.get("console"), bool) or not isinstance(log_cfg.get("file"), bool):
        raise ValueError("log.console and log.file must be booleans")
    if not isinstance(log_cfg.get("file_path"), str) or not log_cfg["file_path"].strip():
        raise ValueError("log.file_path must be a non-empty string")
    validate_log_file_path(log_cfg["file_path"])


def validate_log_file_path(file_path):
    normalized_log_path = os.path.normpath(file_path)
    if (
        os.path.isabs(normalized_log_path)
        or os.path.splitdrive(normalized_log_path)[0]
        or normalized_log_path in {".", os.pardir}
        or normalized_log_path.startswith(os.pardir + os.sep)
    ):
        raise ValueError("log.file_path must stay inside the logs directory")


def apply_config(cfg):
    global HOST, PORT, ENGINE_PATH, HASH, DRAIN_BANNER
    global ALLOWED_ORIGINS, MAX_CONNECTIONS
    global LOG_CONSOLE, LOG_FILE, LOG_FILE_PATH

    HOST = cfg["host"]
    PORT = cfg["port"]
    ENGINE_PATH = cfg["engine_path"]
    HASH = cfg["hash_mb"]
    DRAIN_BANNER = cfg["drain_banner"]
    ALLOWED_ORIGINS = [origin.rstrip("/") for origin in cfg["allowed_origins"]]
    MAX_CONNECTIONS = cfg["max_connections"]
    LOG_CONSOLE = cfg["log"]["console"]
    LOG_FILE = cfg["log"]["file"]
    LOG_FILE_PATH = cfg["log"]["file_path"]


def resolve_log_path(file_path):
    """把配置中的相对文件名固定解析到脚本目录下的 logs/。"""
    validate_log_file_path(file_path)
    return os.path.join(script_dir(), LOG_DIRNAME, os.path.normpath(file_path))


def _open_log_file():
    if not LOG_FILE:
        return None
    log_path = resolve_log_path(LOG_FILE_PATH)
    os.makedirs(os.path.dirname(log_path), exist_ok=True)
    return open(log_path, "a", encoding="utf-8", errors="ignore")


def log_print(message):
    if LOG_CONSOLE:
        print(message, flush=True)
    if _log_file_handle is not None:
        _log_file_handle.write(message + "\n")
        _log_file_handle.flush()


def rewrite_command(message):
    """只精确改写 Hash setoption，避免误伤其它包含相同文本的命令。"""
    if HASH_COMMAND_RE.fullmatch(message):
        return f"setoption name Hash value {HASH}", True
    return message, False


def start_engine():
    return subprocess.Popen(
        [ENGINE_PATH],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        bufsize=1,
        encoding="utf-8",
        errors="ignore",
    )


async def drain_banner(proc):
    loop = asyncio.get_running_loop()
    line = await loop.run_in_executor(None, proc.stdout.readline)
    if line:
        log_print(f"[Engine banner] {line.strip()}")


async def pump_engine_stdout(proc, ws):
    loop = asyncio.get_running_loop()
    try:
        while True:
            line = await loop.run_in_executor(None, proc.stdout.readline)
            if not line:
                with contextlib.suppress(Exception):
                    await ws.send("info string engine exited")
                    await ws.close(code=1011, reason="engine exited")
                return
            line = line.rstrip("\r\n")
            log_print(f"[ENGINE -> WEB] {line}")
            await ws.send(line)
    except asyncio.CancelledError:
        raise
    except Exception as error:
        log_print(f"[ERR] pump stdout: {error}")
        with contextlib.suppress(Exception):
            await ws.close(code=1011, reason="engine output failed")


async def stop_engine(proc):
    if proc.poll() is None:
        with contextlib.suppress(Exception):
            proc.terminate()
    try:
        await asyncio.wait_for(asyncio.to_thread(proc.wait), timeout=3)
    except asyncio.TimeoutError:
        with contextlib.suppress(Exception):
            proc.kill()
        await asyncio.to_thread(proc.wait)


async def ws_handler(ws):
    global _active_connections

    if _active_connections >= MAX_CONNECTIONS:
        log_print(f"[WARN] connection limit reached ({MAX_CONNECTIONS})")
        await ws.close(code=1013, reason="too many local engine sessions")
        return

    _active_connections += 1
    proc = None
    pump_task = None
    log_print(f"[WS] client connected ({_active_connections}/{MAX_CONNECTIONS})")

    try:
        try:
            proc = start_engine()
        except Exception as error:
            log_print(f"[ERR] start engine: {error}")
            await ws.close(code=1011, reason="cannot start engine")
            return

        if DRAIN_BANNER:
            await drain_banner(proc)

        pump_task = asyncio.create_task(pump_engine_stdout(proc, ws))

        async for raw_message in ws:
            message = str(raw_message).strip()
            if not message:
                continue

            log_print(f"[WEB -> ENGINE] {message}")
            message, rewritten = rewrite_command(message)
            if rewritten:
                log_print(f"[INFO] force Hash to {HASH} MB")

            try:
                proc.stdin.write(message + "\n")
                proc.stdin.flush()
            except Exception as error:
                log_print(f"[ERR] write to engine failed: {error}")
                await ws.close(code=1011, reason="engine input failed")
                break
    except Exception as error:
        # 正常断线也会表现为 ConnectionClosed；日志保留但不输出 traceback。
        log_print(f"[WS] connection ended: {error}")
    finally:
        if proc is not None:
            await stop_engine(proc)
        if pump_task is not None:
            pump_task.cancel()
            with contextlib.suppress(asyncio.CancelledError):
                await pump_task
        _active_connections -= 1
        log_print(f"[WS] client disconnected -> engine cleaned ({_active_connections}/{MAX_CONNECTIONS})")


async def run_server():
    # None 允许无 Origin 的本机原生客户端；浏览器必须匹配白名单。
    origins = [
        None,
        *(re.compile(rf"^{re.escape(origin.rstrip('/'))}/?$", re.IGNORECASE) for origin in ALLOWED_ORIGINS),
    ]
    log_print(f"[Python] ws server listening on ws://{HOST}:{PORT}")
    log_print(f"[Python] allowed origins: {', '.join(ALLOWED_ORIGINS)}")
    log_print(f"[Python] max connections: {MAX_CONNECTIONS}")
    async with websockets.serve(
        ws_handler,
        HOST,
        PORT,
        origins=origins,
        compression=None,
        max_size=64 * 1024,
    ):
        await asyncio.Future()


def main():
    global _log_file_handle

    try:
        cfg, path, created = load_config()
        if created:
            print(f"[INFO] wrote default config: {path}")
            print("[INFO] edit engine_path if needed, then restart")
            return
        validate_config(cfg)
        apply_config(cfg)
        _log_file_handle = _open_log_file()
        asyncio.run(run_server())
    except (OSError, ValueError, json.JSONDecodeError) as error:
        print(f"[FATAL] {error}", file=sys.stderr)
        raise SystemExit(1) from error
    finally:
        if _log_file_handle is not None:
            with contextlib.suppress(Exception):
                _log_file_handle.close()


if __name__ == "__main__":
    main()


# test fen：曾观察到车九平四和马七进五轮流出现
# 2baka3/r4n3/1cn1b2c1/p1p1p3p/9/2P1P1r2/P7P/1CN1C1N2/R8/2BAKABR1 w
