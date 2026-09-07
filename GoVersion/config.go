package main

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed config.example.json
var embeddedConfigExample []byte

// LogConfig 日志相关配置。
type LogConfig struct {
	// Console 是否输出到终端。
	Console bool `json:"console"`
	// File 是否输出到文件。
	File bool `json:"file"`
	// FilePath 日志文件相对 logs 目录的路径。
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
	// Port WS 监听端口，与 ../tampermonkey/pkf-web2local.js 中的 WS_URL 一致。
	Port int `json:"port"`
	// EnginePath Pikafish 引擎可执行文件绝对路径。
	EnginePath string `json:"engine_path"`
	// HashMB 强制覆写的 Hash 大小（MB）。网页代码对 >384MB 只会传 384，
	// 这里统一改成目标值，比如 512。
	HashMB int `json:"hash_mb"`
	// DrainBanner 是否吞掉引擎启动后的第一行 banner（Pikafish 会输出
	// "Pikafish ..." 之类的版本信息，避免污染 UCI 流）。
	DrainBanner bool `json:"drain_banner"`
	// AllowedOrigins 是允许发起浏览器 WebSocket 握手的网页 Origin。
	// 无 Origin 的本机原生客户端仍允许连接。
	AllowedOrigins []string `json:"allowed_origins"`
	// MaxConnections 限制同时运行的引擎进程数量，避免网页误操作耗尽内存。
	MaxConnections int `json:"max_connections"`
	// Log 日志配置。
	Log LogConfig `json:"log"`
}

// 默认配置常量。集中放这里便于一眼看到所有可调项。
const (
	defaultHost           = "localhost"
	defaultPort           = 8765
	defaultHashMB         = 512
	defaultDrainBanner    = true
	defaultMaxConnections = 4
	defaultLogConsole     = true
	defaultLogFile        = false
	defaultLogFilePath    = "pkf-local-go.log"
)

// ConfigFileName 配置文件名，与可执行文件同目录。
const ConfigFileName = "config.json"

// ConfigExampleFileName 是可编辑的配置模板文件名。
const ConfigExampleFileName = "config.example.json"

// DefaultConfig 返回默认配置（用于生成示例文件）。
func DefaultConfig() Config {
	return Config{
		Host:           defaultHost,
		Port:           defaultPort,
		EnginePath:     "C:/path/to/pikafish-bmi2.exe",
		HashMB:         defaultHashMB,
		DrainBanner:    defaultDrainBanner,
		AllowedOrigins: []string{"https://xiangqiai.com"},
		MaxConnections: defaultMaxConnections,
		Log: LogConfig{
			Console:  defaultLogConsole,
			File:     defaultLogFile,
			FilePath: defaultLogFilePath,
		},
	}
}

// exeDir 返回当前可执行文件所在目录。go run 的可执行文件位于临时
// go-build 目录，此时改用 cwd，保证开发模式可以稳定读取 GoVersion/config.json。
func exeDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if relative, relErr := filepath.Rel(os.TempDir(), filepath.Dir(exe)); relErr == nil &&
		relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		firstPart := strings.Split(relative, string(filepath.Separator))[0]
		if strings.HasPrefix(strings.ToLower(firstPart), "go-build") {
			return os.Getwd()
		}
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

func loadExampleConfig(dir string) (Config, error) {
	data, err := os.ReadFile(filepath.Join(dir, ConfigExampleFileName))
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return Config{}, fmt.Errorf("read config template: %w", err)
		}
		data = embeddedConfigExample
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", ConfigExampleFileName, err)
	}
	return cfg, nil
}

// LoadConfig 加载配置：
//
//  1. 找 exe 同目录下的 config.json；
//  2. 优先读取同目录 config.example.json，不存在则使用嵌入 EXE 的模板；
//  3. config.json 不存在时根据模板生成；
//  4. config.json 存在时读取，缺失字段由模板补齐。
func LoadConfig() (Config, string, bool, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, "", false, err
	}

	exampleCfg, err := loadExampleConfig(filepath.Dir(path))
	if err != nil {
		return Config{}, "", false, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			if werr := writeConfig(path, exampleCfg); werr != nil {
				return Config{}, "", false, fmt.Errorf("write default config: %w", werr)
			}
			return exampleCfg, path, true, nil
		}
		return Config{}, "", false, fmt.Errorf("read config: %w", err)
	}

	cfg := exampleCfg
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, "", false, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, path, false, nil
}

// normalizeEnginePath 清理引号、环境变量和路径分隔符，并转成绝对路径。
// filepath.FromSlash 让 Windows 同时接受 C:/... 和 C:\...。
func normalizeEnginePath(rawPath string) (string, error) {
	value := strings.TrimSpace(rawPath)
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '"' || first == '\'') && first == last {
			value = strings.TrimSpace(value[1 : len(value)-1])
		}
	}
	if value == "" {
		return "", errors.New("engine path cannot be empty")
	}
	value = os.ExpandEnv(value)
	value = filepath.FromSlash(value)
	absPath, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absPath), nil
}

func engineFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// EnsureEnginePath 在配置路径无效时从终端读取路径，验证后写回 config.json。
func EnsureEnginePath(cfg *Config, configPath string, input io.Reader, output io.Writer) error {
	if normalized, err := normalizeEnginePath(cfg.EnginePath); err == nil && engineFileExists(normalized) {
		cfg.EnginePath = normalized
		return nil
	}

	fmt.Fprintf(output, "[WARN] Pikafish engine not found: %s\n", cfg.EnginePath)
	scanner := bufio.NewScanner(input)
	for {
		fmt.Fprint(output, "请输入 Pikafish 可执行文件路径（支持 / 或 \\，可拖入文件）: ")
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("read engine path: %w", err)
			}
			return errors.New("engine_path is invalid and no interactive input is available")
		}

		candidate, err := normalizeEnginePath(scanner.Text())
		if err != nil {
			fmt.Fprintf(output, "[WARN] %v\n", err)
			continue
		}
		if !engineFileExists(candidate) {
			fmt.Fprintf(output, "[WARN] file does not exist: %s\n", candidate)
			continue
		}

		cfg.EnginePath = candidate
		if err := writeConfig(configPath, *cfg); err != nil {
			return fmt.Errorf("save engine_path: %w", err)
		}
		fmt.Fprintf(output, "[INFO] saved engine_path to: %s\n", configPath)
		return nil
	}
}

// ValidateConfig 在打开监听端口前校验所有会影响安全和运行的字段。
func ValidateConfig(cfg Config) error {
	if strings.TrimSpace(cfg.Host) == "" {
		return errors.New("host must be a non-empty string")
	}
	host := strings.ToLower(strings.TrimSpace(cfg.Host))
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return errors.New("host must be a loopback address (localhost, 127.0.0.1, or ::1)")
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	if cfg.HashMB <= 0 {
		return errors.New("hash_mb must be positive")
	}
	if cfg.MaxConnections < 1 || cfg.MaxConnections > 32 {
		return errors.New("max_connections must be between 1 and 32")
	}
	if len(cfg.AllowedOrigins) == 0 {
		return errors.New("allowed_origins must not be empty")
	}
	for _, origin := range cfg.AllowedOrigins {
		if strings.TrimSpace(origin) == "" {
			return errors.New("allowed_origins must contain only non-empty strings")
		}
	}
	if !filepath.IsAbs(cfg.EnginePath) {
		return errors.New("engine_path must be an absolute path")
	}
	info, err := os.Stat(cfg.EnginePath)
	if err != nil {
		return fmt.Errorf("engine_path is not accessible: %w", err)
	}
	if info.IsDir() {
		return errors.New("engine_path must point to a file")
	}
	if strings.TrimSpace(cfg.Log.FilePath) == "" {
		return errors.New("log.file_path must be a non-empty string")
	}
	if err := validateLogFilePath(cfg.Log.FilePath); err != nil {
		return err
	}
	return nil
}

func validateLogFilePath(filePath string) error {
	cleanLogPath := filepath.Clean(filePath)
	if filepath.IsAbs(cleanLogPath) || filepath.VolumeName(cleanLogPath) != "" ||
		cleanLogPath == "." || cleanLogPath == ".." ||
		strings.HasPrefix(cleanLogPath, ".."+string(filepath.Separator)) {
		return errors.New("log.file_path must stay inside the logs directory")
	}
	return nil
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
