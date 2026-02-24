package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config Agent 配置
type Config struct {
	// 服务器配置
	Management ManagementConfig `yaml:"management"`
	Signal     SignalConfig     `yaml:"signal"`

	// WireGuard 配置
	WireGuard WireGuardConfig `yaml:"wireguard"`

	// NAT 探测配置
	NAT NATConfig `yaml:"nat"`

	// 日志配置
	Log LogConfig `yaml:"log"`

	// 数据目录
	DataDir string `yaml:"data_dir"`
}

// ManagementConfig Management 服务配置
type ManagementConfig struct {
	ServerAddr        string        `yaml:"server_addr"`        // 服务器地址
	ConnectTimeout    time.Duration `yaml:"connect_timeout"`    // 连接超时
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"` // 心跳间隔
}

// SignalConfig Signal 服务配置
type SignalConfig struct {
	ServerAddr     string        `yaml:"server_addr"`     // 服务器地址
	ConnectTimeout time.Duration `yaml:"connect_timeout"` // 连接超时
}

// WireGuardConfig WireGuard 配置
type WireGuardConfig struct {
	ListenPort int `yaml:"listen_port"` // 监听端口（0 表示自动选择）
	MTU        int `yaml:"mtu"`         // MTU 大小
}

// NATConfig NAT 探测配置
type NATConfig struct {
	STUNServers   []string      `yaml:"stun_servers"`   // STUN 服务器列表
	ProbeTimeout  time.Duration `yaml:"probe_timeout"`  // 探测超时
	ProbeInterval time.Duration `yaml:"probe_interval"` // 探测间隔
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string `yaml:"level"`  // 日志级别: debug, info, warn, error
	Format string `yaml:"format"` // 日志格式: text, json
	File   string `yaml:"file"`   // 日志文件路径（空表示输出到 stdout）
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Management: ManagementConfig{
			ServerAddr:        "localhost:8080",
			ConnectTimeout:    10 * time.Second,
			HeartbeatInterval: 30 * time.Second,
		},
		Signal: SignalConfig{
			ServerAddr:     "localhost:8081",
			ConnectTimeout: 10 * time.Second,
		},
		WireGuard: WireGuardConfig{
			ListenPort: 0, // 自动选择
			MTU:        1420,
		},
		NAT: NATConfig{
			STUNServers: []string{
				"stun.l.google.com:19302",
				"stun.cloudflare.com:3478",
			},
			ProbeTimeout:  5 * time.Second,
			ProbeInterval: 5 * time.Minute,
		},
		Log: LogConfig{
			Level:  "info",
			Format: "text",
			File:   "",
		},
		DataDir: defaultDataDir(),
	}
}

// defaultDataDir 返回默认数据目录
func defaultDataDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "/var/lib/go-vpc"
	}
	return filepath.Join(homeDir, ".go-vpc")
}

// Load 从文件加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return cfg, nil
}

// LoadOrDefault 从文件加载配置，如果文件不存在则返回默认配置
func LoadOrDefault(path string) (*Config, error) {
	if path == "" {
		return DefaultConfig(), nil
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	return Load(path)
}

// Save 保存配置到文件
func (c *Config) Save(path string) error {
	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Management.ServerAddr == "" {
		return fmt.Errorf("management.server_addr 不能为空")
	}

	if c.Signal.ServerAddr == "" {
		return fmt.Errorf("signal.server_addr 不能为空")
	}

	if c.WireGuard.MTU < 576 || c.WireGuard.MTU > 65535 {
		return fmt.Errorf("wireguard.mtu 必须在 576-65535 之间")
	}

	if len(c.NAT.STUNServers) == 0 {
		return fmt.Errorf("nat.stun_servers 不能为空")
	}

	return nil
}

// EnsureDataDir 确保数据目录存在
func (c *Config) EnsureDataDir() error {
	return os.MkdirAll(c.DataDir, 0700)
}

// IdentityPath 返回身份文件路径
func (c *Config) IdentityPath() string {
	return filepath.Join(c.DataDir, "identity.json")
}
