package client

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/example/go-vpc/api/signal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/proto"
)

// SignalClient Signal 服务客户端
type SignalClient struct {
	conn   *grpc.ClientConn
	client signal.SignalServiceClient

	deviceID string

	// 信令流
	stream    signal.SignalService_ConnectClient
	streamMu  sync.Mutex
	streamCtx context.Context
	cancel    context.CancelFunc

	// 消息处理
	onMessage func(msg *signal.SignalMessage)
	pendingMu sync.RWMutex
	pending   map[string]chan *signal.SignalMessage // messageID -> response channel
}

// SignalClientConfig 客户端配置
type SignalClientConfig struct {
	ServerAddr     string        // 服务器地址
	DeviceID       string        // 本设备 ID
	ConnectTimeout time.Duration // 连接超时
}

// NewSignalClient 创建 Signal 客户端
func NewSignalClient(cfg SignalClientConfig) (*SignalClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()), // TODO: 生产环境使用 TLS
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             3 * time.Second,
			PermitWithoutStream: true,
		}),
	}

	conn, err := grpc.DialContext(ctx, cfg.ServerAddr, opts...)
	if err != nil {
		return nil, fmt.Errorf("连接 Signal 服务器失败: %w", err)
	}

	return &SignalClient{
		conn:     conn,
		client:   signal.NewSignalServiceClient(conn),
		deviceID: cfg.DeviceID,
		pending:  make(map[string]chan *signal.SignalMessage),
	}, nil
}

// Connect 建立信令通道
func (c *SignalClient) Connect(ctx context.Context) error {
	c.streamMu.Lock()
	defer c.streamMu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}

	c.streamCtx, c.cancel = context.WithCancel(ctx)

	stream, err := c.client.Connect(c.streamCtx)
	if err != nil {
		return fmt.Errorf("建立信令通道失败: %w", err)
	}
	c.stream = stream

	// 启动接收协程
	go c.recvLoop()

	return nil
}

// recvLoop 接收消息循环
func (c *SignalClient) recvLoop() {
	for {
		select {
		case <-c.streamCtx.Done():
			return
		default:
			c.streamMu.Lock()
			stream := c.stream
			c.streamMu.Unlock()

			if stream == nil {
				return
			}

			msg, err := stream.Recv()
			if err != nil {
				return
			}

			// 检查是否是对 pending 消息的响应
			c.pendingMu.RLock()
			ch, ok := c.pending[msg.MessageId]
			c.pendingMu.RUnlock()

			if ok {
				select {
				case ch <- msg:
				default:
				}
			} else if c.onMessage != nil {
				// 处理主动推送的消息
				c.onMessage(msg)
			}
		}
	}
}

// SendOffer 发送连接请求
func (c *SignalClient) SendOffer(ctx context.Context, targetDeviceID string, payload *signal.OfferPayload) (*signal.SignalMessage, error) {
	payloadBytes, err := proto.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("序列化 Offer 失败: %w", err)
	}

	return c.sendAndWait(ctx, targetDeviceID, signal.SignalType_SIGNAL_TYPE_OFFER, payloadBytes)
}

// SendAnswer 发送连接响应
func (c *SignalClient) SendAnswer(ctx context.Context, targetDeviceID, messageID string, payload *signal.AnswerPayload) error {
	payloadBytes, err := proto.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化 Answer 失败: %w", err)
	}

	return c.send(&signal.SignalMessage{
		FromDeviceId: c.deviceID,
		ToDeviceId:   targetDeviceID,
		Type:         signal.SignalType_SIGNAL_TYPE_ANSWER,
		Payload:      payloadBytes,
		Timestamp:    time.Now().UnixMilli(),
		MessageId:    messageID,
	})
}

// SendPunch 发送打洞请求
func (c *SignalClient) SendPunch(ctx context.Context, targetDeviceID string, payload *signal.PunchPayload) error {
	payloadBytes, err := proto.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化 Punch 失败: %w", err)
	}

	return c.send(&signal.SignalMessage{
		FromDeviceId: c.deviceID,
		ToDeviceId:   targetDeviceID,
		Type:         signal.SignalType_SIGNAL_TYPE_PUNCH,
		Payload:      payloadBytes,
		Timestamp:    time.Now().UnixMilli(),
		MessageId:    generateMessageID(),
	})
}

// SendPunchAck 发送打洞确认
func (c *SignalClient) SendPunchAck(ctx context.Context, targetDeviceID string, payload *signal.PunchAckPayload) error {
	payloadBytes, err := proto.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化 PunchAck 失败: %w", err)
	}

	return c.send(&signal.SignalMessage{
		FromDeviceId: c.deviceID,
		ToDeviceId:   targetDeviceID,
		Type:         signal.SignalType_SIGNAL_TYPE_PUNCH_ACK,
		Payload:      payloadBytes,
		Timestamp:    time.Now().UnixMilli(),
		MessageId:    generateMessageID(),
	})
}

// send 发送消息
func (c *SignalClient) send(msg *signal.SignalMessage) error {
	c.streamMu.Lock()
	defer c.streamMu.Unlock()

	if c.stream == nil {
		return fmt.Errorf("信令通道未建立")
	}

	return c.stream.Send(msg)
}

// sendAndWait 发送消息并等待响应
func (c *SignalClient) sendAndWait(ctx context.Context, targetDeviceID string, msgType signal.SignalType, payload []byte) (*signal.SignalMessage, error) {
	messageID := generateMessageID()

	// 创建响应通道
	ch := make(chan *signal.SignalMessage, 1)
	c.pendingMu.Lock()
	c.pending[messageID] = ch
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, messageID)
		c.pendingMu.Unlock()
	}()

	// 发送消息
	err := c.send(&signal.SignalMessage{
		FromDeviceId: c.deviceID,
		ToDeviceId:   targetDeviceID,
		Type:         msgType,
		Payload:      payload,
		Timestamp:    time.Now().UnixMilli(),
		MessageId:    messageID,
	})
	if err != nil {
		return nil, err
	}

	// 等待响应
	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SetMessageHandler 设置消息处理回调
func (c *SignalClient) SetMessageHandler(handler func(msg *signal.SignalMessage)) {
	c.onMessage = handler
}

// Close 关闭客户端
func (c *SignalClient) Close() error {
	c.streamMu.Lock()
	if c.cancel != nil {
		c.cancel()
	}
	c.stream = nil
	c.streamMu.Unlock()

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// messageCounter 用于生成唯一消息 ID 的原子计数器
var messageCounter uint64

// generateMessageID 生成消息 ID（时间戳 + 原子计数器，确保唯一性）
func generateMessageID() string {
	counter := atomic.AddUint64(&messageCounter, 1)
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), counter)
}
