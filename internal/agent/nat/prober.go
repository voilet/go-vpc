package nat

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/pion/stun"
)

// Prober 负责 NAT 探测
type Prober struct {
	stunServers []string
	timeout     time.Duration
}

// NewProber 创建新的 NAT 探测器
func NewProber(stunServers []string, timeout time.Duration) *Prober {
	return &Prober{
		stunServers: stunServers,
		timeout:     timeout,
	}
}

// Probe 执行 NAT 探测
func (p *Prober) Probe(ctx context.Context) (*Result, error) {
	var lastErr error

	for _, server := range p.stunServers {
		result, err := p.probeServer(ctx, server)
		if err != nil {
			lastErr = err
			continue
		}
		return result, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("所有 STUN 服务器探测失败: %w", lastErr)
	}
	return nil, fmt.Errorf("没有可用的 STUN 服务器")
}

// probeServer 向单个 STUN 服务器发送探测请求
func (p *Prober) probeServer(ctx context.Context, server string) (*Result, error) {
	// 创建 UDP 连接
	conn, err := net.DialTimeout("udp", server, p.timeout)
	if err != nil {
		return nil, fmt.Errorf("连接 STUN 服务器失败: %w", err)
	}
	defer conn.Close()

	// 设置超时
	deadline, ok := ctx.Deadline()
	if ok {
		conn.SetDeadline(deadline)
	} else {
		conn.SetDeadline(time.Now().Add(p.timeout))
	}

	// 构建 STUN Binding Request
	message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)

	// 发送请求
	if _, err := conn.Write(message.Raw); err != nil {
		return nil, fmt.Errorf("发送 STUN 请求失败: %w", err)
	}

	// 读取响应
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("读取 STUN 响应失败: %w", err)
	}

	// 解析响应
	response := &stun.Message{Raw: buf[:n]}
	if err := response.Decode(); err != nil {
		return nil, fmt.Errorf("解析 STUN 响应失败: %w", err)
	}

	// 提取 XOR-MAPPED-ADDRESS
	var xorAddr stun.XORMappedAddress
	if err := xorAddr.GetFrom(response); err != nil {
		// 尝试获取 MAPPED-ADDRESS
		var mappedAddr stun.MappedAddress
		if err := mappedAddr.GetFrom(response); err != nil {
			return nil, fmt.Errorf("无法获取映射地址: %w", err)
		}
		return &Result{
			PublicAddr: fmt.Sprintf("%s:%d", mappedAddr.IP, mappedAddr.Port),
			NATType:    p.detectNATType(conn.LocalAddr().String(), mappedAddr.IP.String()),
			LocalAddr:  conn.LocalAddr().String(),
		}, nil
	}

	return &Result{
		PublicAddr: fmt.Sprintf("%s:%d", xorAddr.IP, xorAddr.Port),
		NATType:    p.detectNATType(conn.LocalAddr().String(), xorAddr.IP.String()),
		LocalAddr:  conn.LocalAddr().String(),
	}, nil
}

// detectNATType 检测 NAT 类型（简化版）
// 完整实现需要多次探测不同服务器
func (p *Prober) detectNATType(localAddr, publicIP string) NATType {
	// 获取本地 IP
	host, _, _ := net.SplitHostPort(localAddr)
	localIP := net.ParseIP(host)

	// 如果本地 IP 就是公网 IP，说明没有 NAT
	if localIP != nil && localIP.String() == publicIP {
		return NATTypeNone
	}

	// 简化实现：默认返回端口受限锥形
	// 完整实现需要向多个服务器/端口发送请求并对比结果
	return NATTypePortRestricted
}
