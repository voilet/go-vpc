package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/example/go-vpc/internal/agent"
	"github.com/example/go-vpc/internal/agent/config"
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := config.LoadOrDefault(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听系统信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\n收到停止信号，正在优雅退出...")
		cancel()
	}()

	// 创建并启动 Agent
	a, err := agent.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建 Agent 失败: %v\n", err)
		os.Exit(1)
	}

	if err := a.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Agent 运行错误: %v\n", err)
		os.Exit(1)
	}
}
