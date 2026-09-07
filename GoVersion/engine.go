package main

import (
	"bufio"
	"errors"
	"io"
	"log"
	"os/exec"
	"strings"
)

// Engine 封装一个 Pikafish 子进程：stdin 用于写入 UCI 命令，
// stdout 通过 bufio.Reader 按行读取。
//
// 一个 Engine 实例对应一个连接；连接关闭时由 Kill() 强制结束子进程。
type Engine struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *bufio.Reader
}

// StartEngine 启动指定路径的 Pikafish 引擎子进程并返回封装。
//
// stderr 合并到 stdout；stdin/stdout 都用 OS pipe，不经文本模式，
// 避免 Windows 上 \n <-> \r\n 的隐式转换。
func StartEngine(path string) (*Engine, error) {
	cmd := exec.Command(path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &Engine{
		cmd:    cmd,
		stdin:  stdin,
		reader: bufio.NewReader(stdout),
	}, nil
}

// DrainBanner 吞掉引擎启动后输出的第一行 banner（Pikafish 通常是版本信息）。
//
// 对应 Python 版的 drain_banner(p)。这里用 bufio.ReadString('\n') 等价。
func (e *Engine) DrainBanner() (string, error) {
	line, err := e.reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	trimmed := strings.TrimRight(line, "\r\n")
	if trimmed != "" {
		log.Printf("[Engine banner] %s", trimmed)
	}
	return trimmed, nil
}

// Send 把一条 UCI 命令写入子进程 stdin，附 '\n'。
//
// 对应 Python 版: proc.stdin.write(msg + "\n"); proc.stdin.flush()
// Pipe 本身不带缓冲，WriteString 立即进入 OS pipe，不需要显式 flush。
func (e *Engine) Send(line string) error {
	_, err := io.WriteString(e.stdin, line+"\n")
	return err
}

// ReadLine 阻塞读取一行 UCI 输出。
//
//   - 正常返回 (line, nil)，line 不含尾部换行
//   - 读到 EOF 且有未换行残留时，返回 (line, io.EOF)
//   - 读到 EOF 且没有数据时，返回 ("", io.EOF)
//   - 其它错误：返回 ("", err)
func (e *Engine) ReadLine() (string, error) {
	line, err := e.reader.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) {
			if line != "" {
				return strings.TrimRight(line, "\r\n"), io.EOF
			}
			return "", io.EOF
		}
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// Kill 强制结束子进程并 Wait 回收，避免 Windows 上残留僵尸进程。
//
// 对应 Python 版: proc.terminate()，Windows 上 TerminateProcess 是硬杀。
func (e *Engine) Kill() {
	if e.cmd == nil || e.cmd.Process == nil {
		return
	}
	_ = e.cmd.Process.Kill()
	// Wait 必须在 Kill 之后调用，否则 Windows 上会留下句柄不释放。
	_ = e.cmd.Wait()
}