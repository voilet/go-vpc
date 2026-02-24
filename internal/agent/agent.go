package agent

import (
	"context"
	"fmt"
)

// Agent 是客户端代理的主结构
type Agent struct {
	// 后续添加各模块
}

// New 创建新的 Agent 实例
func New() (*Agent, error) {
	return &Agent{}, nil
}

// Start 启动 Agent
func (a *Agent) Start(ctx context.Context) error {
	fmt.Println("Agent 启动中...")

	// 等待上下文取消
	<-ctx.Done()
	fmt.Println("Agent 已停止")
	return nil
}

// Stop 停止 Agent
func (a *Agent) Stop() error {
	return nil
}
