package identity

import (
	"crypto/ed25519"
	"encoding/base64"
)

// Identity 表示设备身份
type Identity struct {
	DeviceID    string             // 设备唯一标识（公钥的 SHA256 哈希）
	Fingerprint string             // 设备指纹
	PrivateKey  ed25519.PrivateKey // Ed25519 私钥
	PublicKey   ed25519.PublicKey  // Ed25519 公钥
}

// PublicKeyBase64 返回 Base64 编码的公钥
func (i *Identity) PublicKeyBase64() string {
	return base64.StdEncoding.EncodeToString(i.PublicKey)
}

// PrivateKeyBase64 返回 Base64 编码的私钥
func (i *Identity) PrivateKeyBase64() string {
	return base64.StdEncoding.EncodeToString(i.PrivateKey)
}
