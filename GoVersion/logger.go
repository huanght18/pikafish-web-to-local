package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

// setupLogger 根据 LogConfig 配置全局 log 输出：
//   - Console=true：写到 stdout
//   - File=true：附加写到 exe 所在目录的 logs/ 下（追加模式）
//   - 两者皆 false：写到 io.Discard（静默）
//
// 返回的关闭函数会在程序退出前被调用，用于关闭日志文件句柄。
func setupLogger(logCfg LogConfig) (func() error, error) {
	var writers []io.Writer

	if logCfg.Console {
		writers = append(writers, os.Stdout)
	}

	var f *os.File
	if logCfg.File {
		path, err := resolveLogPath(logCfg.FilePath)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		f, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, err
		}
		writers = append(writers, f)
	}

	if len(writers) == 0 {
		log.SetOutput(io.Discard)
	} else if len(writers) == 1 {
		log.SetOutput(writers[0])
	} else {
		log.SetOutput(io.MultiWriter(writers...))
	}

	return func() error {
		if f != nil {
			return f.Close()
		}
		return nil
	}, nil
}

// resolveLogPath 将配置中的相对路径固定到 exe 目录下的 logs/。
func resolveLogPath(filePath string) (string, error) {
	if filePath == "" {
		filePath = defaultLogFilePath
	}
	if err := validateLogFilePath(filePath); err != nil {
		return "", err
	}
	dir, err := exeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "logs", filepath.Clean(filePath)), nil
}
