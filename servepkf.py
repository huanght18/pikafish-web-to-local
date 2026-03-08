import asyncio
import subprocess
import websockets

# ENGINE_PATH = r"D:\Disk\Pikafish-20260102\pikafish-bmi2.exe"
ENGINE_PATH = r"D:\Disk\Pikafish-20260131\pikafish-bmi2.exe"
HOST = "localhost"
PORT = 8765

proc: subprocess.Popen | None = None
stdout_queue: asyncio.Queue[str] | None = None


def start_engine() -> subprocess.Popen:
    """启动引擎子进程（同步函数）"""
    p = subprocess.Popen(
        ENGINE_PATH,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        bufsize=1,          # 行缓冲（对实时输出很关键）
        encoding="utf-8",
        errors="ignore",
    )
    return p


async def drain_banner(p: subprocess.Popen):
    """吞掉启动时的第一行 banner，避免污染协议流"""
    loop = asyncio.get_running_loop()
    line = await loop.run_in_executor(None, p.stdout.readline)
    if line:
        print("[Engine banner]", line.strip())


async def pump_engine_stdout(p: subprocess.Popen, q: asyncio.Queue[str]):
    """后台任务：持续读取引擎 stdout，放入队列（不直接发给任何客户端）"""
    loop = asyncio.get_running_loop()
    while True:
        line = await loop.run_in_executor(None, p.stdout.readline)
        if not line:
            await q.put("info string engine exited")
            break
        await q.put(line.rstrip("\n"))


async def ws_handler(ws):
    global proc, stdout_queue
    assert proc is not None and stdout_queue is not None

    print("[WS] client connected")

    # 把 stdout 队列持续转发给这个客户端
    async def forward_stdout():
        while True:
            line = await stdout_queue.get()
            print("[ENGINE -> WEB]", line)
            await ws.send(line)

    forward_task = asyncio.create_task(forward_stdout())

    try:
        async for msg in ws:
            msg = msg.strip()
            if not msg:
                continue

            print("[WEB -> ENGINE]", msg)
            try:
                proc.stdin.write(msg + "\n")
                proc.stdin.flush()
            except Exception as e:
                print("[ERR] write to engine failed:", e)
                break
    finally:
        forward_task.cancel()
        print("[WS] client disconnected")


async def main():
    global proc, stdout_queue

    print("[Python] starting engine...")
    proc = start_engine()
    stdout_queue = asyncio.Queue()

    # 1) 过滤 banner
    await drain_banner(proc)

    # 2) 启动 stdout 泵（常驻）
    asyncio.create_task(pump_engine_stdout(proc, stdout_queue))

    # 3) 等待浏览器连接
    print(f"[Python] ws server listening on ws://{HOST}:{PORT}")
    async with websockets.serve(ws_handler, HOST, PORT):
        await asyncio.Future()


if __name__ == "__main__":
    try:
        asyncio.run(main())
    finally:
        if proc is not None:
            try:
                proc.terminate()
            except Exception:
                pass
