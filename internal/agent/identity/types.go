package identity

import (
	"crypto/ed25519"
	"encoding/base64"
)

// Identity 表示设备身份
type Identity struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
}

// PublicKeyBase64 返回 Base64 编码的公钥
func (i *Identity) PublicKeyBase64() string {
	return base64.StdEncoding.EncodeToString(i.PublicKey)
}

// PrivateKeyBase64 返回 Base64 编码的私钥
func (i *Identity) PrivateKeyBase64() string {
	return base64.StdEncoding.EncodeToString(i.PrivateKey)
}
