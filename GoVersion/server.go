package main

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// 全局 upgrader：本地工具，Origin 校验不做强制要求。
// 如要收紧可改成判断 r.Header.Get("Origin") 前缀。
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// handlerTimeout 控制 WS upgrade 的握手超时。
const handlerTimeout = 10 * time.Second

// cfg 是从 config.json 加载的全局配置，由 main 在启动时注入。
var cfg Config

// applyConfig 在 main 中初始化 cfg。
func applyConfig(c Config) {
	cfg = c
}

// wsHandler 每个浏览器标签（WS 连接）都会进入这里一次，
// 各自启动一个 Pikafish 引擎子进程，输入/输出双向转发。
//
// 这是 servepkf2.py 中 ws_handler 的 Go 对应实现。
func wsHandler(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ERR] upgrade: %v", err)
		return
	}
	// 进程退出时确保关闭 WS（读协程也会兜底）。
	defer func() { _ = ws.Close() }()

	log.Println("[WS] client connected")

	eng, err := StartEngine(cfg.EnginePath)
	if err != nil {
		log.Printf("[ERR] start engine: %v", err)
		return
	}

	if cfg.DrainBanner {
		if _, err := eng.DrainBanner(); err != nil {
			log.Printf("[WARN] drain banner: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	// goroutine 1: engine stdout -> browser
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			line, err := eng.ReadLine()
			if line != "" {
				log.Printf("[ENGINE -> WEB] %s", line)
				if werr := ws.WriteMessage(websocket.TextMessage, []byte(line)); werr != nil {
					log.Printf("[ERR] ws write: %v", werr)
					cancel()
					return
				}
			}
			if err != nil {
				// 引擎 EOF 或异常退出，通知浏览器后退出本 goroutine。
				_ = ws.WriteMessage(websocket.TextMessage, []byte("info string engine exited"))
				cancel()
				return
			}
		}
	}()

	// goroutine 2: browser -> engine stdin
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				log.Printf("[ERR] ws read: %v", err)
				cancel()
				return
			}
			line := strings.TrimSpace(string(msg))
			if line == "" {
				continue
			}
			log.Printf("[WEB -> ENGINE] %s", line)

			// 强制覆写 Hash 值：与 servepkf2.py 一致。
			if strings.Contains(line, "setoption name Hash value") {
				line = "setoption name Hash value " + strconv.Itoa(cfg.HashMB)
				log.Printf("[INFO] detected Hash option, force switching to %d", cfg.HashMB)
			}

			if err := eng.Send(line); err != nil {
				log.Printf("[ERR] engine write: %v", err)
				cancel()
				return
			}
		}
	}()

	// 任意一端先挂掉，ctx 就会被 cancel，结束等待。
	<-ctx.Done()

	// 关闭 WS（让 ReadMessage 返回错误），再杀引擎，最后回收协程。
	_ = ws.Close()
	eng.Kill()
	wg.Wait()

	log.Println("[WS] client disconnected -> engine killed")
}