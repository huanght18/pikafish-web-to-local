package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

// setupLogger 根据 LogConfig 配置全局 log 输出：
//   - Console=true：写到 stdout
//   - File=true：附加写到 exe 同目录下的日志文件（追加模式）
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
		path := logCfg.FilePath
		if path == "" {
			path = "pkf-local-go.log"
		}
		if !filepath.IsAbs(path) {
			dir, err := exeDir()
			if err != nil {
				return nil, err
			}
			path = filepath.Join(dir, path)
		}
		var err error
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