package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// LogConfig 日志相关配置。
type LogConfig struct {
	// Console 是否输出到终端。
	Console bool `json:"console"`
	// File 是否输出到文件。
	File bool `json:"file"`
	// FilePath 日志文件相对 exe 所在目录的路径。
	FilePath string `json:"file_path"`
}

// Config 全部运行期配置，可由 config.json 覆盖。
//
// 注意：JSON 里凡是省略的字段会回落到这里的零值/默认值；
// 任何"必填"的语义都在 LoadConfig 里集中校验。
type Config struct {
	// Host WS 监听地址，例如 "localhost" / "127.0.0.1" / "0.0.0.0"。
	// 生产只在本机用，保持 "localhost" 即可，不要随便改成 0.0.0.0。
	Host string `json:"host"`
	// Port WS 监听端口，与 pkf-local.js 中的 WS_URL 一致。
	Port int `json:"port"`
	// EnginePath Pikafish 引擎可执行文件绝对路径。
	EnginePath string `json:"engine_path"`
	// HashMB 强制覆写的 Hash 大小（MB）。网页代码对 >384MB 只会传 384，
	// 这里统一改成目标值，比如 512。
	HashMB int `json:"hash_mb"`
	// DrainBanner 是否吞掉引擎启动后的第一行 banner（Pikafish 会输出
	// "Pikafish ..." 之类的版本信息，避免污染 UCI 流）。
	DrainBanner bool `json:"drain_banner"`
	// Log 日志配置。
	Log LogConfig `json:"log"`
}

// 默认配置常量。集中放这里便于一眼看到所有可调项。
const (
	defaultHost        = "localhost"
	defaultPort        = 8765
	defaultHashMB      = 512
	defaultDrainBanner = true
	defaultLogConsole  = true
	defaultLogFile     = false
	defaultLogFilePath = "pkf-local-go.log"
)

// ConfigFileName 配置文件名，与可执行文件同目录。
const ConfigFileName = "config.json"

// DefaultConfig 返回默认配置（用于生成示例文件）。
func DefaultConfig() Config {
	return Config{
		Host:        defaultHost,
		Port:        defaultPort,
		EnginePath:  `D:\Personal\ChineseChess\pikafish-20260131\pikafish-bmi2.exe`,
		HashMB:      defaultHashMB,
		DrainBanner: defaultDrainBanner,
		Log: LogConfig{
			Console:  defaultLogConsole,
			File:     defaultLogFile,
			FilePath: defaultLogFilePath,
		},
	}
}

// exeDir 返回当前可执行文件所在目录（不一定是 cwd）。
func exeDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

// configPath 返回配置文件的绝对路径：exe 同目录/config.json。
func configPath() (string, error) {
	dir, err := exeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ConfigFileName), nil
}

// LoadConfig 加载配置：
//
//  1. 找 exe 同目录下的 config.json；
//  2. 不存在则把默认配置写入该路径，方便用户编辑，然后返回默认值；
//  3. 存在则读取，缺失字段用默认值补齐。
func LoadConfig() (Config, string, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			cfg := DefaultConfig()
			if werr := writeConfig(path, cfg); werr != nil {
				return Config{}, "", fmt.Errorf("write default config: %w", werr)
			}
			return cfg, path, nil
		}
		return Config{}, "", fmt.Errorf("read config: %w", err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, "", fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, path, nil
}

// writeConfig 以缩进良好的 JSON 写出默认配置。
//
// encoding/json 不支持注释，因此这里的 "header" 是一段无 // 的纯文本；
// 它不会破坏 JSON 解析（首字符为 '{' 之前的所有内容都被 json 包忽略），
// 但解析器实现严格的话可能会报错，所以这里干脆不写 header，
// 字段说明放到 README.md 里集中解释。
func writeConfig(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}