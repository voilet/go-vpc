package agent

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/example/go-vpc/api/management"
	"github.com/example/go-vpc/api/signal"
	"github.com/example/go-vpc/internal/agent/client"
	"github.com/example/go-vpc/internal/agent/config"
	"github.com/example/go-vpc/internal/agent/identity"
	"github.com/example/go-vpc/internal/agent/nat"
	"github.com/example/go-vpc/internal/agent/wireguard"
)

// Agent 是客户端代理的主结构
type Agent struct {
	cfg      *config.Config
	identity *identity.Identity

	// 客户端
	mgmtClient   *client.ManagementClient
	signalClient *client.SignalClient

	// WireGuard 引擎
	wgEngine *wireguard.UserspaceEngine

	// NAT 探测器
	natProber *nat.Prober
	natResult *nat.Result

	// 网络信息
	vip       net.IP
	vipPrefix int
	networkID string

	// 状态
	mu      sync.RWMutex
	running bool
	cancel  context.CancelFunc
}

// New 创建新的 Agent 实例
func New(cfg *config.Config) (*Agent, error) {
	// 确保数据目录存在
	if err := cfg.EnsureDataDir(); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	// 加载或创建身份
	id, err := identity.LoadOrCreate(cfg.IdentityPath())
	if err != nil {
		return nil, fmt.Errorf("加载身份失败: %w", err)
	}

	return &Agent{
		cfg:       cfg,
		identity:  id,
		wgEngine:  wireguard.NewUserspaceEngine(),
		natProber: nat.NewProber(cfg.NAT.STUNServers, cfg.NAT.ProbeTimeout),
	}, nil
}

// Start 启动 Agent
func (a *Agent) Start(ctx context.Context) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("Agent 已在运行")
	}
	a.running = true
	ctx, a.cancel = context.WithCancel(ctx)
	a.mu.Unlock()

	log.Printf("Agent 启动中... DeviceID: %s", a.identity.DeviceID)

	// 1. NAT 探测
	if err := a.probeNAT(ctx); err != nil {
		log.Printf("警告: NAT 探测失败: %v", err)
	}

	// 2. 连接 Management 服务器
	if err := a.connectManagement(ctx); err != nil {
		return fmt.Errorf("连接 Management 服务器失败: %w", err)
	}

	// 3. 注册设备
	if err := a.register(ctx); err != nil {
		return fmt.Errorf("设备注册失败: %w", err)
	}

	// 4. 初始化 WireGuard
	if err := a.initWireGuard(); err != nil {
		return fmt.Errorf("初始化 WireGuard 失败: %w", err)
	}

	// 5. 连接 Signal 服务器
	if err := a.connectSignal(ctx); err != nil {
		return fmt.Errorf("连接 Signal 服务器失败: %w", err)
	}

	// 6. 启动心跳
	if err := a.startHeartbeat(ctx); err != nil {
		return fmt.Errorf("启动心跳失败: %w", err)
	}

	// 7. 启动 Peer 同步
	go a.syncPeers(ctx)

	// 8. 启动定期 NAT 探测
	go a.periodicNATProbe(ctx)

	log.Printf("Agent 已启动，VIP: %s/%d", a.vip, a.vipPrefix)

	// 等待上下文取消
	<-ctx.Done()
	return a.shutdown()
}

// probeNAT NAT 探测
func (a *Agent) probeNAT(ctx context.Context) error {
	result, err := a.natProber.Probe(ctx)
	if err != nil {
		return err
	}

	a.mu.Lock()
	a.natResult = result
	a.mu.Unlock()

	log.Printf("NAT 探测结果: %s", result)
	return nil
}

// connectManagement 连接 Management 服务器
func (a *Agent) connectManagement(ctx context.Context) error {
	mgmtClient, err := client.NewManagementClient(client.ManagementClientConfig{
		ServerAddr:        a.cfg.Management.ServerAddr,
		ConnectTimeout:    a.cfg.Management.ConnectTimeout,
		HeartbeatInterval: a.cfg.Management.HeartbeatInterval,
	})
	if err != nil {
		return err
	}

	a.mgmtClient = mgmtClient

	// 设置命令处理回调
	a.mgmtClient.SetCommandHandler(a.handleCommand)

	return nil
}

// register 注册设备
func (a *Agent) register(ctx context.Context) error {
	resp, err := a.mgmtClient.Register(ctx, &management.RegisterRequest{
		DeviceId:    a.identity.DeviceID,
		PublicKey:   a.identity.PublicKeyBase64(),
		Fingerprint: a.identity.Fingerprint,
		DeviceInfo: &management.DeviceInfo{
			Hostname: getHostname(),
			Os:       getOS(),
			Arch:     getArch(),
			Version:  "0.1.0",
		},
	})
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("注册失败: %s", resp.Message)
	}

	// 保存网络信息
	a.vip = net.ParseIP(resp.AssignedVip)
	a.vipPrefix = int(resp.VipPrefix)
	a.networkID = resp.NetworkId

	log.Printf("设备注册成功，分配 VIP: %s/%d", a.vip, a.vipPrefix)
	return nil
}

// initWireGuard 初始化 WireGuard
func (a *Agent) initWireGuard() error {
	return a.wgEngine.Init(wireguard.Config{
		PrivateKey: a.identity.PrivateKeyBase64(),
		ListenPort: a.cfg.WireGuard.ListenPort,
		MTU:        a.cfg.WireGuard.MTU,
	}, a.vip, a.vipPrefix)
}

// connectSignal 连接 Signal 服务器
func (a *Agent) connectSignal(ctx context.Context) error {
	signalClient, err := client.NewSignalClient(client.SignalClientConfig{
		ServerAddr:     a.cfg.Signal.ServerAddr,
		DeviceID:       a.identity.DeviceID,
		ConnectTimeout: a.cfg.Signal.ConnectTimeout,
	})
	if err != nil {
		return err
	}

	a.signalClient = signalClient

	// 设置消息处理回调
	a.signalClient.SetMessageHandler(a.handleSignalMessage)

	// 建立信令通道
	return a.signalClient.Connect(ctx)
}

// startHeartbeat 启动心跳
func (a *Agent) startHeartbeat(ctx context.Context) error {
	return a.mgmtClient.StartHeartbeat(ctx, a.identity.DeviceID, a.cfg.Management.HeartbeatInterval)
}

// syncPeers 同步 Peer 列表
func (a *Agent) syncPeers(ctx context.Context) {
	err := a.mgmtClient.SyncPeers(ctx, a.identity.DeviceID, a.networkID, func(update *management.PeerUpdate) {
		a.handlePeerUpdate(update)
	})
	if err != nil && ctx.Err() == nil {
		log.Printf("Peer 同步错误: %v", err)
	}
}

// periodicNATProbe 定期 NAT 探测
func (a *Agent) periodicNATProbe(ctx context.Context) {
	ticker := time.NewTicker(a.cfg.NAT.ProbeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.probeNAT(ctx); err != nil {
				log.Printf("定期 NAT 探测失败: %v", err)
			}
		}
	}
}

// handleCommand 处理服务端下发的命令
func (a *Agent) handleCommand(cmd *management.Command) {
	switch cmd.Type {
	case management.CommandType_COMMAND_TYPE_UPDATE_PEERS:
		log.Printf("收到更新 Peer 命令")
		// TODO: 解析并处理 Peer 更新
	case management.CommandType_COMMAND_TYPE_RECONNECT:
		log.Printf("收到重连命令")
		// TODO: 执行重连逻辑
	default:
		log.Printf("收到未知命令类型: %v", cmd.Type)
	}
}

// handleSignalMessage 处理信令消息
func (a *Agent) handleSignalMessage(msg *signal.SignalMessage) {
	// TODO: 实现信令消息处理
	log.Printf("收到信令消息: from=%s, type=%v", msg.FromDeviceId, msg.Type)
}

// handlePeerUpdate 处理 Peer 更新
func (a *Agent) handlePeerUpdate(update *management.PeerUpdate) {
	peer := update.Peer
	if peer == nil {
		return
	}

	switch update.Type {
	case management.PeerUpdateType_PEER_UPDATE_TYPE_ADD:
		log.Printf("添加 Peer: %s (%s)", peer.DeviceId, peer.Vip)
		endpoint := ""
		if len(peer.Endpoints) > 0 {
			endpoint = peer.Endpoints[0]
		}
		if err := a.wgEngine.AddPeer(wireguard.PeerConfig{
			PublicKey:  peer.PublicKey,
			Endpoint:   endpoint,
			AllowedIPs: peer.AllowedIps,
		}); err != nil {
			log.Printf("添加 Peer 失败: %v", err)
		}

	case management.PeerUpdateType_PEER_UPDATE_TYPE_REMOVE:
		log.Printf("移除 Peer: %s", peer.DeviceId)
		if err := a.wgEngine.RemovePeer(peer.PublicKey); err != nil {
			log.Printf("移除 Peer 失败: %v", err)
		}

	case management.PeerUpdateType_PEER_UPDATE_TYPE_UPDATE:
		log.Printf("更新 Peer: %s", peer.DeviceId)
		if len(peer.Endpoints) > 0 {
			if err := a.wgEngine.UpdatePeerEndpoint(peer.PublicKey, peer.Endpoints[0]); err != nil {
				log.Printf("更新 Peer 端点失败: %v", err)
			}
		}
	}
}

// shutdown 关闭 Agent
func (a *Agent) shutdown() error {
	log.Println("Agent 正在关闭...")

	// 停止心跳
	if a.mgmtClient != nil {
		a.mgmtClient.StopHeartbeat()
	}

	// 关闭 Signal 客户端
	if a.signalClient != nil {
		a.signalClient.Close()
	}

	// 关闭 Management 客户端
	if a.mgmtClient != nil {
		a.mgmtClient.Close()
	}

	// 关闭 WireGuard 引擎
	if a.wgEngine != nil {
		a.wgEngine.Close()
	}

	a.mu.Lock()
	a.running = false
	a.mu.Unlock()

	log.Println("Agent 已停止")
	return nil
}

// Stop 停止 Agent
func (a *Agent) Stop() error {
	a.mu.Lock()
	cancel := a.cancel
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	return nil
}

// 辅助函数
func getHostname() string {
	// 简化实现
	return "unknown"
}

func getOS() string {
	return "unknown"
}

func getArch() string {
	return "unknown"
}
