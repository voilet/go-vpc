package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/curve25519"
)

// Generate 生成新的密钥对（Ed25519 用于设备身份，Curve25519 用于 WireGuard）
func Generate() (*Identity, error) {
	// 生成 Ed25519 密钥对（用于设备身份）
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成 Ed25519 密钥对失败: %w", err)
	}

	// 生成 Curve25519 密钥对（用于 WireGuard）
	var wgPrivKey [32]byte
	if _, err := rand.Read(wgPrivKey[:]); err != nil {
		return nil, fmt.Errorf("生成 WireGuard 私钥失败: %w", err)
	}
	// Clamp 私钥（WireGuard 规范）
	wgPrivKey[0] &= 248
	wgPrivKey[31] &= 127
	wgPrivKey[31] |= 64

	var wgPubKey [32]byte
	curve25519.ScalarBaseMult(&wgPubKey, &wgPrivKey)

	// 生成 DeviceID（Ed25519 公钥的 SHA256 哈希）
	hash := sha256.Sum256(pub)
	deviceID := hex.EncodeToString(hash[:])

	// 生成设备指纹
	fingerprint, err := GenerateFingerprint()
	if err != nil {
		return nil, fmt.Errorf("生成设备指纹失败: %w", err)
	}

	return &Identity{
		DeviceID:     deviceID,
		Fingerprint:  fingerprint,
		PrivateKey:   priv,
		PublicKey:    pub,
		WGPrivateKey: wgPrivKey,
		WGPublicKey:  wgPubKey,
	}, nil
}

// identityFile 身份文件结构（用于 JSON 序列化）
type identityFile struct {
	Ed25519PrivateKey []byte `json:"ed25519_private_key"`
	WGPrivateKey      []byte `json:"wg_private_key"`
}

// Save 将身份保存到文件（JSON 格式）
func Save(id *Identity, path string) error {
	data := identityFile{
		Ed25519PrivateKey: id.PrivateKey,
		WGPrivateKey:      id.WGPrivateKey[:],
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化身份失败: %w", err)
	}

	if err := os.WriteFile(path, jsonData, 0600); err != nil {
		return fmt.Errorf("写入身份文件失败: %w", err)
	}

	return nil
}

// Load 从文件加载身份
func Load(path string) (*Identity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取身份文件失败: %w", err)
	}

	var file identityFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("解析身份文件失败: %w", err)
	}

	// 恢复 Ed25519 密钥对
	priv := ed25519.PrivateKey(file.Ed25519PrivateKey)
	pub := priv.Public().(ed25519.PublicKey)

	// 恢复 WireGuard 密钥对
	var wgPrivKey [32]byte
	copy(wgPrivKey[:], file.WGPrivateKey)

	var wgPubKey [32]byte
	curve25519.ScalarBaseMult(&wgPubKey, &wgPrivKey)

	// 生成 DeviceID（Ed25519 公钥的 SHA256 哈希）
	hash := sha256.Sum256(pub)
	deviceID := hex.EncodeToString(hash[:])

	// 生成设备指纹
	fingerprint, err := GenerateFingerprint()
	if err != nil {
		return nil, fmt.Errorf("生成设备指纹失败: %w", err)
	}

	return &Identity{
		DeviceID:     deviceID,
		Fingerprint:  fingerprint,
		PrivateKey:   priv,
		PublicKey:    pub,
		WGPrivateKey: wgPrivKey,
		WGPublicKey:  wgPubKey,
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
