package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Management.ServerAddr == "" {
		t.Error("Management.ServerAddr 不应为空")
	}

	if cfg.Signal.ServerAddr == "" {
		t.Error("Signal.ServerAddr 不应为空")
	}

	if cfg.WireGuard.MTU != 1420 {
		t.Errorf("WireGuard.MTU 应为 1420，实际为 %d", cfg.WireGuard.MTU)
	}

	if len(cfg.NAT.STUNServers) == 0 {
		t.Error("NAT.STUNServers 不应为空")
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("默认配置验证失败: %v", err)
	}
}

func TestConfigSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// 创建配置
	cfg := DefaultConfig()
	cfg.Management.ServerAddr = "test.example.com:8080"
	cfg.Management.HeartbeatInterval = 60 * time.Second

	// 保存配置
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("保存配置失败: %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("配置文件不存在")
	}

	// 加载配置
	loadedCfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	// 验证值
	if loadedCfg.Management.ServerAddr != "test.example.com:8080" {
		t.Errorf("ServerAddr 不匹配: %s", loadedCfg.Management.ServerAddr)
	}

	if loadedCfg.Management.HeartbeatInterval != 60*time.Second {
		t.Errorf("HeartbeatInterval 不匹配: %v", loadedCfg.Management.HeartbeatInterval)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name:    "默认配置有效",
			modify:  func(c *Config) {},
			wantErr: false,
		},
		{
			name:    "空 Management 地址",
			modify:  func(c *Config) { c.Management.ServerAddr = "" },
			wantErr: true,
		},
		{
			name:    "空 Signal 地址",
			modify:  func(c *Config) { c.Signal.ServerAddr = "" },
			wantErr: true,
		},
		{
			name:    "MTU 过小",
			modify:  func(c *Config) { c.WireGuard.MTU = 100 },
			wantErr: true,
		},
		{
			name:    "MTU 过大",
			modify:  func(c *Config) { c.WireGuard.MTU = 100000 },
			wantErr: true,
		},
		{
			name:    "空 STUN 服务器",
			modify:  func(c *Config) { c.NAT.STUNServers = nil },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)

			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadOrDefault(t *testing.T) {
	// 空路径返回默认配置
	cfg, err := LoadOrDefault("")
	if err != nil {
		t.Fatalf("LoadOrDefault 失败: %v", err)
	}
	if cfg == nil {
		t.Fatal("配置为空")
	}

	// 不存在的文件返回默认配置
	cfg, err = LoadOrDefault("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("LoadOrDefault 失败: %v", err)
	}
	if cfg == nil {
		t.Fatal("配置为空")
	}
}

func TestEnsureDataDir(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := DefaultConfig()
	cfg.DataDir = filepath.Join(tmpDir, "test-data")

	if err := cfg.EnsureDataDir(); err != nil {
		t.Fatalf("EnsureDataDir 失败: %v", err)
	}

	if _, err := os.Stat(cfg.DataDir); os.IsNotExist(err) {
		t.Fatal("数据目录不存在")
	}
}

func TestIdentityPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DataDir = "/test/data"

	expected := "/test/data/identity.json"
	if cfg.IdentityPath() != expected {
		t.Errorf("IdentityPath() = %s, want %s", cfg.IdentityPath(), expected)
	}
}
