// pkf-local-go：把 main.py 的功能用 Go 重写（原文件名 servepkf2.py）。
//
// 工作流：
//  1. 浏览器端的油猴脚本 ../tampermonkey/pkf-web2local.js 拦截 window.Pikafish.sendCommand，
//     把每条 UCI 命令作为一条 text message 发到 ws://<host>:<port>。
//  2. 本服务为每条 WS 连接 fork 一个 Pikafish 引擎子进程，
//     双向转发 UCI 命令与引擎输出，关闭时清理子进程。
//
// 与 main.py 行为对齐（原文件名 servepkf2.py）：每连接独立引擎、吞 banner、Hash 覆写。
// 配置通过同目录下的 config.json 加载；首次启动会自动生成示例文件。
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	log.SetFlags(0)

	// 命令行 flag 覆盖配置文件中的 host / port，方便临时调试。
	// engine_path / hash_mb / drain_banner / log.* 仅通过 config.json 配置。
	var (
		flagHost = flag.String("host", "", "WS 监听地址覆盖配置 host")
		flagPort = flag.Int("port", 0, "WS 监听端口覆盖配置 port")
	)
	flag.Parse()

	cfg, path, created, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] load config: %v\n", err)
		os.Exit(1)
	}
	if created {
		fmt.Printf("[INFO] created config from %s: %s\n", ConfigExampleFileName, path)
	}
	if err := EnsureEnginePath(&cfg, path, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] configure engine_path: %v\n", err)
		os.Exit(1)
	}

	if *flagHost != "" {
		log.Printf("[Go] flag override host: %s -> %s", cfg.Host, *flagHost)
		cfg.Host = *flagHost
	}
	if *flagPort != 0 {
		log.Printf("[Go] flag override port: %d -> %d", cfg.Port, *flagPort)
		cfg.Port = *flagPort
	}
	if err := ValidateConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] invalid config: %v\n", err)
		os.Exit(1)
	}

	closeLogger, err := setupLogger(cfg.Log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] setup logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = closeLogger() }()

	applyConfig(cfg)
	log.Printf("[Go] config loaded: %s", path)

	mux := http.NewServeMux()
	mux.HandleFunc("/", wsHandler)
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: handlerTimeout,
	}

	// 用 goroutine 跑 ListenenAndServe，主线程捕获 Ctrl+C / SIGTERM 后
	// 给 server 5 秒时间做优雅关闭（虽然只是关 listener，子进程由
	// 各 wsHandler 在 WS 断开时自行清理）。
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("[Go] ws server listening on ws://%s", addr)
		log.Printf("[Go] engine: %s", cfg.EnginePath)
		log.Printf("[Go] hash override: %d MB", cfg.HashMB)
		log.Printf("[Go] allowed origins: %v", cfg.AllowedOrigins)
		log.Printf("[Go] max connections: %d", cfg.MaxConnections)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
		close(serverErr)
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-stop:
		log.Printf("[Go] received %v, shutting down...", sig)
	case err := <-serverErr:
		if err != nil {
			log.Printf("[ERR] listen: %v", err)
		}
	}

	// http.Server.Shutdown 不会关闭 WebSocket 这类 hijacked 连接，先显式
	// 关闭它们，触发各 handler 杀死并回收对应的引擎进程。
	shutdownWebSockets()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("[WARN] server shutdown: %v", err)
	}
	if !waitForConnections(ctx) {
		log.Printf("[WARN] timed out waiting for engine sessions to stop")
	}
	log.Println("[Go] bye")
}
