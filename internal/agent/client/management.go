package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/example/go-vpc/api/management"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// ManagementClient Management 服务客户端
type ManagementClient struct {
	conn   *grpc.ClientConn
	client management.ManagementServiceClient

	// 心跳相关
	heartbeatStream management.ManagementService_HeartbeatClient
	heartbeatMu     sync.Mutex
	heartbeatCancel context.CancelFunc

	// 回调函数
	onCommand func(cmd *management.Command)
}

// ManagementClientConfig 客户端配置
type ManagementClientConfig struct {
	ServerAddr       string        // 服务器地址
	ConnectTimeout   time.Duration // 连接超时
	HeartbeatInterval time.Duration // 心跳间隔
}

// NewManagementClient 创建 Management 客户端
func NewManagementClient(cfg ManagementClientConfig) (*ManagementClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	// 配置 gRPC 连接
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()), // TODO: 生产环境使用 TLS
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second, // 发送 keepalive ping 的间隔
			Timeout:             3 * time.Second,  // 等待 keepalive ping 响应的超时
			PermitWithoutStream: true,             // 没有活跃流时也发送 ping
		}),
	}

	conn, err := grpc.DialContext(ctx, cfg.ServerAddr, opts...)
	if err != nil {
		return nil, fmt.Errorf("连接 Management 服务器失败: %w", err)
	}

	return &ManagementClient{
		conn:   conn,
		client: management.NewManagementServiceClient(conn),
	}, nil
}

// Register 注册设备
func (c *ManagementClient) Register(ctx context.Context, req *management.RegisterRequest) (*management.RegisterResponse, error) {
	return c.client.Register(ctx, req)
}

// GetNetworkConfig 获取网络配置
func (c *ManagementClient) GetNetworkConfig(ctx context.Context, deviceID, networkID string) (*management.NetworkConfig, error) {
	return c.client.GetNetworkConfig(ctx, &management.GetNetworkConfigRequest{
		DeviceId:  deviceID,
		NetworkId: networkID,
	})
}

// StartHeartbeat 启动心跳
func (c *ManagementClient) StartHeartbeat(ctx context.Context, deviceID string, interval time.Duration) error {
	c.heartbeatMu.Lock()
	defer c.heartbeatMu.Unlock()

	// 取消之前的心跳
	if c.heartbeatCancel != nil {
		c.heartbeatCancel()
	}

	heartbeatCtx, cancel := context.WithCancel(ctx)
	c.heartbeatCancel = cancel

	stream, err := c.client.Heartbeat(heartbeatCtx)
	if err != nil {
		cancel()
		return fmt.Errorf("建立心跳流失败: %w", err)
	}
	c.heartbeatStream = stream

	// 启动发送协程
	go c.heartbeatSendLoop(heartbeatCtx, deviceID, interval)

	// 启动接收协程
	go c.heartbeatRecvLoop(heartbeatCtx)

	return nil
}

// heartbeatSendLoop 心跳发送循环
func (c *ManagementClient) heartbeatSendLoop(ctx context.Context, deviceID string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.heartbeatMu.Lock()
			if c.heartbeatStream != nil {
				err := c.heartbeatStream.Send(&management.HeartbeatRequest{
					DeviceId:  deviceID,
					Timestamp: time.Now().UnixMilli(),
				})
				if err != nil {
					// 心跳发送失败，可以尝试重连
					c.heartbeatMu.Unlock()
					return
				}
			}
			c.heartbeatMu.Unlock()
		}
	}
}

// heartbeatRecvLoop 心跳接收循环
func (c *ManagementClient) heartbeatRecvLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			c.heartbeatMu.Lock()
			stream := c.heartbeatStream
			c.heartbeatMu.Unlock()

			if stream == nil {
				return
			}

			resp, err := stream.Recv()
			if err != nil {
				// 流断开
				return
			}

			// 处理服务端下发的命令
			for _, cmd := range resp.Commands {
				if c.onCommand != nil {
					c.onCommand(cmd)
				}
			}
		}
	}
}

// StopHeartbeat 停止心跳
func (c *ManagementClient) StopHeartbeat() {
	c.heartbeatMu.Lock()
	defer c.heartbeatMu.Unlock()

	if c.heartbeatCancel != nil {
		c.heartbeatCancel()
		c.heartbeatCancel = nil
	}
	c.heartbeatStream = nil
}

// SetCommandHandler 设置命令处理回调
func (c *ManagementClient) SetCommandHandler(handler func(cmd *management.Command)) {
	c.onCommand = handler
}

// SyncPeers 同步 Peer 列表
func (c *ManagementClient) SyncPeers(ctx context.Context, deviceID, networkID string, handler func(update *management.PeerUpdate)) error {
	stream, err := c.client.SyncPeers(ctx, &management.SyncPeersRequest{
		DeviceId:  deviceID,
		NetworkId: networkID,
	})
	if err != nil {
		return fmt.Errorf("建立 Peer 同步流失败: %w", err)
	}

	for {
		update, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("接收 Peer 更新失败: %w", err)
		}
		handler(update)
	}
}

// Close 关闭客户端
func (c *ManagementClient) Close() error {
	c.StopHeartbeat()
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
