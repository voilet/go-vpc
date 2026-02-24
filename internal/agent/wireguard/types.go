package wireguard

import "net"

// Config WireGuard 配置
type Config struct {
	PrivateKey string // Base64 编码的私钥
	ListenPort int    // 监听端口
	MTU        int    // MTU 大小
}

// PeerConfig Peer 配置
type PeerConfig struct {
	PublicKey  string   // Base64 编码的公钥
	Endpoint   string   // 对端地址 IP:Port
	AllowedIPs []string // 允许的 IP 范围
}

// Engine WireGuard 引擎接口
type Engine interface {
	// Init 初始化引擎
	Init(cfg Config, localVIP net.IP, vipPrefix int) error

	// AddPeer 添加 Peer
	AddPeer(peer PeerConfig) error

	// RemovePeer 移除 Peer
	RemovePeer(publicKey string) error

	// UpdatePeerEndpoint 更新 Peer 端点
	UpdatePeerEndpoint(publicKey string, endpoint string) error

	// GetListenPort 获取实际监听端口
	GetListenPort() int

	// Close 关闭引擎
	Close() error
}
