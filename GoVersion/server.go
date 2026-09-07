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
	CheckOrigin: func(r *http.Request) bool {
		return originAllowed(r.Header.Get("Origin"))
	},
}

// handlerTimeout 控制 WS upgrade 的握手超时。
const handlerTimeout = 10 * time.Second

// cfg 是从 config.json 加载的全局配置，由 main 在启动时注入。
var cfg Config

var connectionSlots chan struct{}

var connections = struct {
	sync.Mutex
	items        map[*websocket.Conn]struct{}
	shuttingDown bool
}{items: make(map[*websocket.Conn]struct{})}

var connectionWG sync.WaitGroup

// applyConfig 在 main 中初始化 cfg。
func applyConfig(c Config) {
	cfg = c
	connectionSlots = make(chan struct{}, c.MaxConnections)
}

func normalizeOrigin(origin string) string {
	return strings.TrimRight(strings.ToLower(strings.TrimSpace(origin)), "/")
}

func originAllowed(origin string) bool {
	// 非浏览器本机客户端通常没有 Origin；网页连接则必须命中白名单。
	if strings.TrimSpace(origin) == "" {
		return true
	}
	want := normalizeOrigin(origin)
	for _, allowed := range cfg.AllowedOrigins {
		if want == normalizeOrigin(allowed) {
			return true
		}
	}
	return false
}

func rewriteCommand(line string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) >= 5 &&
		strings.EqualFold(fields[0], "setoption") &&
		strings.EqualFold(fields[1], "name") &&
		strings.EqualFold(fields[2], "hash") &&
		strings.EqualFold(fields[3], "value") {
		return "setoption name Hash value " + strconv.Itoa(cfg.HashMB), true
	}
	return line, false
}

func registerConnection(ws *websocket.Conn) bool {
	connections.Lock()
	defer connections.Unlock()
	if connections.shuttingDown {
		return false
	}
	connections.items[ws] = struct{}{}
	connectionWG.Add(1)
	return true
}

func unregisterConnection(ws *websocket.Conn) {
	connections.Lock()
	if _, ok := connections.items[ws]; ok {
		delete(connections.items, ws)
		connectionWG.Done()
	}
	connections.Unlock()
}

// shutdownWebSockets 关闭 http.Server.Shutdown 不会处理的 hijacked WS 连接。
func shutdownWebSockets() {
	connections.Lock()
	connections.shuttingDown = true
	active := make([]*websocket.Conn, 0, len(connections.items))
	for ws := range connections.items {
		active = append(active, ws)
	}
	connections.Unlock()

	for _, ws := range active {
		_ = ws.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutting down"),
			time.Now().Add(time.Second),
		)
		_ = ws.Close()
	}
}

func waitForConnections(ctx context.Context) bool {
	done := make(chan struct{})
	go func() {
		connectionWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	}
}

// wsHandler 每个浏览器标签（WS 连接）都会进入这里一次，
// 各自启动一个 Pikafish 引擎子进程，输入/输出双向转发。
//
// 这是 main.py 中 ws_handler 的 Go 对应实现（原文件名为 servepkf2.py）。
func wsHandler(w http.ResponseWriter, r *http.Request) {
	select {
	case connectionSlots <- struct{}{}:
		defer func() { <-connectionSlots }()
	default:
		log.Printf("[WARN] connection limit reached (%d)", cfg.MaxConnections)
		http.Error(w, "too many local engine sessions", http.StatusServiceUnavailable)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ERR] upgrade: %v", err)
		return
	}
	if !registerConnection(ws) {
		_ = ws.Close()
		return
	}
	defer unregisterConnection(ws)
	// 进程退出时确保关闭 WS（读协程也会兜底）。
	defer func() { _ = ws.Close() }()

	log.Printf("[WS] client connected (%d max)", cfg.MaxConnections)

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

			// 强制覆写 Hash 值：与 main.py 一致（原文件名 servepkf2.py）。
			if rewritten, ok := rewriteCommand(line); ok {
				line = rewritten
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
