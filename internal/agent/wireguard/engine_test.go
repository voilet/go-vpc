package wireguard

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"golang.org/x/crypto/curve25519"
)

// generateKeyPair 生成 WireGuard 密钥对
func generateKeyPair() (privateKey, publicKey string, err error) {
	var privKey [32]byte
	if _, err := rand.Read(privKey[:]); err != nil {
		return "", "", err
	}

	// Clamp 私钥（WireGuard 规范）
	privKey[0] &= 248
	privKey[31] &= 127
	privKey[31] |= 64

	var pubKey [32]byte
	curve25519.ScalarBaseMult(&pubKey, &privKey)

	return base64.StdEncoding.EncodeToString(privKey[:]),
		base64.StdEncoding.EncodeToString(pubKey[:]),
		nil
}

func TestUserspaceEngineInit(t *testing.T) {
	privateKey, _, err := generateKeyPair()
	if err != nil {
		t.Fatalf("生成密钥对失败: %v", err)
	}

	engine := NewUserspaceEngine()

	cfg := Config{
		PrivateKey: privateKey,
		ListenPort: 51820,
		MTU:        1420,
	}

	// 使用测试 VIP
	vip := []byte{10, 200, 0, 1}

	err = engine.Init(cfg, vip, 24)
	if err != nil {
		t.Fatalf("初始化引擎失败: %v", err)
	}
	defer engine.Close()

	if engine.GetListenPort() != 51820 {
		t.Errorf("监听端口不正确: %d", engine.GetListenPort())
	}

	if engine.GetNet() == nil {
		t.Error("netstack 网络接口为空")
	}
}

func TestUserspaceEngineAddRemovePeer(t *testing.T) {
	// 生成本地密钥对
	localPrivKey, _, err := generateKeyPair()
	if err != nil {
		t.Fatalf("生成本地密钥对失败: %v", err)
	}

	// 生成对端密钥对
	_, peerPubKey, err := generateKeyPair()
	if err != nil {
		t.Fatalf("生成对端密钥对失败: %v", err)
	}

	engine := NewUserspaceEngine()

	cfg := Config{
		PrivateKey: localPrivKey,
		ListenPort: 51821,
		MTU:        1420,
	}

	vip := []byte{10, 200, 0, 2}
	err = engine.Init(cfg, vip, 24)
	if err != nil {
		t.Fatalf("初始化引擎失败: %v", err)
	}
	defer engine.Close()

	// 添加 Peer
	peer := PeerConfig{
		PublicKey:  peerPubKey,
		Endpoint:   "192.168.1.100:51820",
		AllowedIPs: []string{"10.200.0.100/32"},
	}

	err = engine.AddPeer(peer)
	if err != nil {
		t.Fatalf("添加 Peer 失败: %v", err)
	}

	// 更新端点
	err = engine.UpdatePeerEndpoint(peerPubKey, "192.168.1.101:51820")
	if err != nil {
		t.Fatalf("更新端点失败: %v", err)
	}

	// 移除 Peer
	err = engine.RemovePeer(peerPubKey)
	if err != nil {
		t.Fatalf("移除 Peer 失败: %v", err)
	}
}

func TestUserspaceEngineInvalidKey(t *testing.T) {
	engine := NewUserspaceEngine()

	cfg := Config{
		PrivateKey: "invalid-base64-key",
		ListenPort: 51822,
		MTU:        1420,
	}

	vip := []byte{10, 200, 0, 3}
	err := engine.Init(cfg, vip, 24)
	if err == nil {
		engine.Close()
		t.Error("期望初始化失败，但成功了")
	}
}
