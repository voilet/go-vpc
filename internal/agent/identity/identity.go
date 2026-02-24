package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

// Generate 生成新的 Ed25519 密钥对
func Generate() (*Identity, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成密钥对失败: %w", err)
	}

	return &Identity{
		PrivateKey: priv,
		PublicKey:  pub,
	}, nil
}

// Save 将身份保存到文件（PEM 格式）
func Save(id *Identity, path string) error {
	block := &pem.Block{
		Type:  "ED25519 PRIVATE KEY",
		Bytes: id.PrivateKey,
	}

	data := pem.EncodeToMemory(block)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("写入密钥文件失败: %w", err)
	}

	return nil
}

// Load 从文件加载身份
func Load(path string) (*Identity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取密钥文件失败: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("无效的 PEM 数据")
	}

	if block.Type != "ED25519 PRIVATE KEY" {
		return nil, fmt.Errorf("不支持的密钥类型: %s", block.Type)
	}

	priv := ed25519.PrivateKey(block.Bytes)
	pub := priv.Public().(ed25519.PublicKey)

	return &Identity{
		PrivateKey: priv,
		PublicKey:  pub,
	}, nil
}

// LoadOrCreate 加载现有身份，如果不存在则创建新的
func LoadOrCreate(path string) (*Identity, error) {
	// 尝试加载
	if _, err := os.Stat(path); err == nil {
		return Load(path)
	}

	// 创建新身份
	id, err := Generate()
	if err != nil {
		return nil, err
	}

	// 确保目录存在
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("创建目录失败: %w", err)
		}
	}

	// 保存
	if err := Save(id, path); err != nil {
		return nil, err
	}

	return id, nil
}
