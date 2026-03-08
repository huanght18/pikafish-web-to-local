# servepkf2.py - 基于 websockets 的 Pikafish 引擎服务器（改进版）
# 2024-06-01 by H18
# 改为每个ws连接对应一个引擎进程，一是多网页各自对应引擎子进程，二是刷新网页引擎重加载而清空Hash表
# 强制覆写Hash表大小
# 因为网页中对于超过384MB的Hash表，即使设置512MB，也会传参为384MB
# TODO: 有时候观察到同样引擎，网页版比tchess的慢，不知道是调度问题还是有天然卡点

import asyncio
import subprocess
import websockets

# ENGINE_PATH = r"D:\Disk\Pikafish-20260131\pikafish-bmi2.exe"
ENGINE_PATH = r"C:\Users\80563\Downloads\ChineseChess\pikafish-20260131\pikafish-bmi2.exe"
HASH = 512
HOST = "localhost"
PORT = 8765


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
    """吞掉启动时的第一行 banner"""
    loop = asyncio.get_running_loop()
    line = await loop.run_in_executor(None, p.stdout.readline)
    if line:
        print("[Engine banner]", line.strip())


async def pump_engine_stdout(p: subprocess.Popen, ws: websockets.WebSocketServerProtocol):
    """持续读取引擎 stdout，并实时推给浏览器"""
    loop = asyncio.get_running_loop()
    try:
        while True:
            line = await loop.run_in_executor(None, p.stdout.readline)
            if not line:
                await ws.send("info string engine exited")
                break

            line = line.rstrip("\n")
            print("[ENGINE -> WEB]", line)
            await ws.send(line)
    except asyncio.CancelledError:
        pass
    except Exception as e:
        print("[ERR] pump stdout:", e)


async def ws_handler(ws):
    print("[WS] client connected")

    # === 每个连接都有自己的引擎进程 ===
    proc = start_engine()

    # 过滤 banner
    await drain_banner(proc)

    # 启动 stdout 转发任务（专属于这个连接）
    pump_task = asyncio.create_task(pump_engine_stdout(proc, ws))

    try:
        async for msg in ws:
            msg = msg.strip()
            if not msg:
                continue

            print("[WEB -> ENGINE]", msg)

            if "setoption name Hash value" in msg:
                print(f"[INFO] detected Hash option, force switching to {HASH}")
                msg = f"setoption name Hash value {HASH}"

            try:
                proc.stdin.write(msg + "\n")
                proc.stdin.flush()
            except Exception as e:
                print("[ERR] write to engine failed:", e)
                break

    finally:
        print("[WS] client disconnected → killing engine")

        # 关闭转发任务
        pump_task.cancel()

        # 杀掉对应引擎进程
        try:
            proc.terminate()
        except Exception:
            pass


async def main():
    print(f"[Python] ws server listening on ws://{HOST}:{PORT}")

    async with websockets.serve(ws_handler, HOST, PORT):
        await asyncio.Future()  # 永久运行


if __name__ == "__main__":
    asyncio.run(main())

# test fen
# 具有现象为车九平四和马七进五轮流出
# 2baka3/r4n3/1cn1b2c1/p1p1p3p/9/2P1P1r2/P7P/1CN1C1N2/R8/2BAKABR1 w