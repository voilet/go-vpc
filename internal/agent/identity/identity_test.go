package identity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateIdentity(t *testing.T) {
	id, err := Generate()
	if err != nil {
		t.Fatalf("生成身份失败: %v", err)
	}

	if len(id.PrivateKey) != 64 {
		t.Errorf("私钥长度错误: got %d, want 64", len(id.PrivateKey))
	}

	if len(id.PublicKey) != 32 {
		t.Errorf("公钥长度错误: got %d, want 32", len(id.PublicKey))
	}

	// 验证 Base64 编码
	if id.PublicKeyBase64() == "" {
		t.Error("PublicKeyBase64 返回空字符串")
	}
}

func TestSaveAndLoadIdentity(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "identity-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	keyPath := filepath.Join(tmpDir, "identity.key")

	// 生成并保存
	id1, err := Generate()
	if err != nil {
		t.Fatalf("生成身份失败: %v", err)
	}

	if err := Save(id1, keyPath); err != nil {
		t.Fatalf("保存身份失败: %v", err)
	}

	// 加载并验证
	id2, err := Load(keyPath)
	if err != nil {
		t.Fatalf("加载身份失败: %v", err)
	}

	if id1.PublicKeyBase64() != id2.PublicKeyBase64() {
		t.Error("加载的公钥与原始不匹配")
	}
}

func TestLoadOrCreate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "identity-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	keyPath := filepath.Join(tmpDir, "identity.key")

	// 首次调用应该创建新身份
	id1, err := LoadOrCreate(keyPath)
	if err != nil {
		t.Fatalf("LoadOrCreate 失败: %v", err)
	}

	// 再次调用应该加载相同身份
	id2, err := LoadOrCreate(keyPath)
	if err != nil {
		t.Fatalf("LoadOrCreate 失败: %v", err)
	}

	if id1.PublicKeyBase64() != id2.PublicKeyBase64() {
		t.Error("两次 LoadOrCreate 返回不同的身份")
	}
}
