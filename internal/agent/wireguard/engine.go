package wireguard

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"sync"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

// UserspaceEngine 用户态 WireGuard 引擎（基于 wireguard-go + netstack）
type UserspaceEngine struct {
	dev    *device.Device
	tun    *netstack.Net
	net    *netstack.Net
	config Config

	listenPort int
	localVIP   net.IP
	vipPrefix  int

	mu      sync.RWMutex
	peers   map[string]*peerState // publicKey -> state
	running bool
}

// peerState 存储 Peer 状态
type peerState struct {
	config   PeerConfig
	endpoint string
}

// NewUserspaceEngine 创建用户态引擎
func NewUserspaceEngine() *UserspaceEngine {
	return &UserspaceEngine{
		peers: make(map[string]*peerState),
	}
}

// Init 初始化引擎
func (e *UserspaceEngine) Init(cfg Config, localVIP net.IP, vipPrefix int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("引擎已在运行")
	}

	e.config = cfg
	e.localVIP = localVIP
	e.vipPrefix = vipPrefix

	// 解析私钥
	privateKey, err := base64.StdEncoding.DecodeString(cfg.PrivateKey)
	if err != nil {
		return fmt.Errorf("解析私钥失败: %w", err)
	}
	if len(privateKey) != 32 {
		return fmt.Errorf("私钥长度错误: %d", len(privateKey))
	}

	// 构建本地地址
	localAddr, err := netip.ParseAddr(localVIP.String())
	if err != nil {
		return fmt.Errorf("解析本地 VIP 失败: %w", err)
	}
	localPrefix := netip.PrefixFrom(localAddr, vipPrefix)

	// 创建 netstack TUN 设备
	tun, tnet, err := netstack.CreateNetTUN(
		[]netip.Addr{localAddr},
		[]netip.Addr{}, // DNS 服务器（暂不配置）
		cfg.MTU,
	)
	if err != nil {
		return fmt.Errorf("创建 netstack TUN 失败: %w", err)
	}
	e.tun = tnet
	e.net = tnet

	// 创建 WireGuard 设备
	logger := device.NewLogger(device.LogLevelSilent, "")
	e.dev = device.NewDevice(tun, conn.NewDefaultBind(), logger)

	// 配置设备
	configStr := fmt.Sprintf("private_key=%s\nlisten_port=%d\n",
		hex.EncodeToString(privateKey),
		cfg.ListenPort,
	)
	if err := e.dev.IpcSet(configStr); err != nil {
		e.dev.Close()
		return fmt.Errorf("配置 WireGuard 设备失败: %w", err)
	}

	// 启动设备
	if err := e.dev.Up(); err != nil {
		e.dev.Close()
		return fmt.Errorf("启动 WireGuard 设备失败: %w", err)
	}

	e.listenPort = cfg.ListenPort
	e.running = true

	_ = localPrefix // 保留，未来可能用于路由配置

	return nil
}

// AddPeer 添加 Peer
func (e *UserspaceEngine) AddPeer(peer PeerConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return fmt.Errorf("引擎未运行")
	}

	// 解析公钥
	publicKey, err := base64.StdEncoding.DecodeString(peer.PublicKey)
	if err != nil {
		return fmt.Errorf("解析公钥失败: %w", err)
	}
	if len(publicKey) != 32 {
		return fmt.Errorf("公钥长度错误: %d", len(publicKey))
	}

	// 构建配置字符串
	configStr := fmt.Sprintf("public_key=%s\n", hex.EncodeToString(publicKey))

	// 添加端点
	if peer.Endpoint != "" {
		configStr += fmt.Sprintf("endpoint=%s\n", peer.Endpoint)
	}

	// 添加允许的 IP 范围
	for _, allowedIP := range peer.AllowedIPs {
		configStr += fmt.Sprintf("allowed_ip=%s\n", allowedIP)
	}

	// 设置 persistent keepalive
	configStr += "persistent_keepalive_interval=25\n"

	if err := e.dev.IpcSet(configStr); err != nil {
		return fmt.Errorf("添加 Peer 失败: %w", err)
	}

	// 记录状态
	e.peers[peer.PublicKey] = &peerState{
		config:   peer,
		endpoint: peer.Endpoint,
	}

	return nil
}

// RemovePeer 移除 Peer
func (e *UserspaceEngine) RemovePeer(publicKey string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return fmt.Errorf("引擎未运行")
	}

	// 解析公钥
	pubKeyBytes, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil {
		return fmt.Errorf("解析公钥失败: %w", err)
	}

	// 移除 Peer
	configStr := fmt.Sprintf("public_key=%s\nremove=true\n", hex.EncodeToString(pubKeyBytes))
	if err := e.dev.IpcSet(configStr); err != nil {
		return fmt.Errorf("移除 Peer 失败: %w", err)
	}

	delete(e.peers, publicKey)

	return nil
}

// UpdatePeerEndpoint 更新 Peer 端点
func (e *UserspaceEngine) UpdatePeerEndpoint(publicKey string, endpoint string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return fmt.Errorf("引擎未运行")
	}

	// 解析公钥
	pubKeyBytes, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil {
		return fmt.Errorf("解析公钥失败: %w", err)
	}

	// 更新端点
	configStr := fmt.Sprintf("public_key=%s\nendpoint=%s\n",
		hex.EncodeToString(pubKeyBytes),
		endpoint,
	)
	if err := e.dev.IpcSet(configStr); err != nil {
		return fmt.Errorf("更新端点失败: %w", err)
	}

	// 更新状态
	if state, ok := e.peers[publicKey]; ok {
		state.endpoint = endpoint
	}

	return nil
}

// GetListenPort 获取实际监听端口
func (e *UserspaceEngine) GetListenPort() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.listenPort
}

// GetNet 获取 netstack 网络接口（用于应用层通信）
func (e *UserspaceEngine) GetNet() *netstack.Net {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.net
}

// Close 关闭引擎
func (e *UserspaceEngine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return nil
	}

	if e.dev != nil {
		e.dev.Close()
		e.dev = nil
	}

	e.running = false
	e.peers = make(map[string]*peerState)

	return nil
}
