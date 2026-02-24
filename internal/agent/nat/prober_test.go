package nat

import (
	"context"
	"testing"
	"time"
)

func TestProberWithPublicSTUN(t *testing.T) {
	// 使用公共 STUN 服务器测试
	prober := NewProber([]string{
		"stun.l.google.com:19302",
		"stun.cloudflare.com:3478",
	}, 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := prober.Probe(ctx)
	if err != nil {
		t.Fatalf("探测失败: %v", err)
	}

	if result.PublicAddr == "" {
		t.Error("未获取到公网地址")
	}

	if result.NATType == NATTypeUnknown {
		t.Log("警告: NAT 类型未知，可能是网络环境问题")
	}

	t.Logf("探测结果: %s", result)
}

func TestProberTimeout(t *testing.T) {
	// 使用不存在的 STUN 服务器测试超时
	prober := NewProber([]string{
		"192.0.2.1:3478", // TEST-NET，不可达
	}, 1*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := prober.Probe(ctx)
	if err == nil {
		t.Error("期望超时错误，但成功返回")
	}
}
