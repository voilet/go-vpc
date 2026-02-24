# Agent P0 实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 实现 Agent 核心功能，支持 1000 节点测试

**Architecture:** 分层模块化架构，采用 TDD 开发。先搭建项目骨架和 Proto 定义，再逐个实现 identity、wireguard、nat、route、client 模块。

**Tech Stack:** Go 1.21+, gRPC, Protocol Buffers, wireguard-go, netlink, pion/stun

---

## 前置准备

### Task 0: 初始化 Go 项目

**Files:**
- Create: `go.mod`
- Create: `cmd/agent/main.go`
- Create: `internal/agent/agent.go`

**Step 1: 初始化 Go module**

```bash
cd /Users/voilet/project/go_prod/src/go-vpc/.worktrees/agent
go mod init github.com/example/go-vpc
```

**Step 2: 创建目录结构**

```bash
mkdir -p cmd/agent
mkdir -p internal/agent/{identity,wireguard,nat,route,network,client,config}
mkdir -p pkg/{proto/management,proto/signal,types}
mkdir -p configs
```

**Step 3: 创建入口文件 cmd/agent/main.go**

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/example/go-vpc/internal/agent"
)

func main() {
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
	a, err := agent.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建 Agent 失败: %v\n", err)
		os.Exit(1)
	}

	if err := a.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Agent 运行错误: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 4: 创建 Agent 主结构 internal/agent/agent.go**

```go
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
```

**Step 5: 验证编译通过**

```bash
go build ./cmd/agent
```

Expected: 编译成功，生成 agent 可执行文件

**Step 6: 提交**

```bash
git add -A
git commit -m "chore: 初始化 Go 项目结构

- 创建 go.mod
- 创建 cmd/agent 入口
- 创建 internal/agent 主结构
- 创建模块目录骨架

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 1: Identity 模块 - 密钥对管理

### Task 1.1: 定义 Identity 接口和类型

**Files:**
- Create: `internal/agent/identity/types.go`
- Create: `internal/agent/identity/identity.go`
- Create: `internal/agent/identity/identity_test.go`

**Step 1: 创建类型定义 internal/agent/identity/types.go**

```go
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
```

**Step 2: 编写失败测试 internal/agent/identity/identity_test.go**

```go
package identity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateIdentity(t *testing.T) {
	id, err := Generate()
	if err != nil {
		t.Fatalf("生成身份失败: %v", err)
	}

	if len(id.PrivateKey) != 64 {
		t.Errorf("私钥长度错误: got %d, want 64", len(id.PrivateKey))
	}

	if len(id.PublicKey) != 32 {
		t.Errorf("公钥长度错误: got %d, want 32", len(id.PublicKey))
	}

	// 验证 Base64 编码
	if id.PublicKeyBase64() == "" {
		t.Error("PublicKeyBase64 返回空字符串")
	}
}

func TestSaveAndLoadIdentity(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "identity-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	keyPath := filepath.Join(tmpDir, "identity.key")

	// 生成并保存
	id1, err := Generate()
	if err != nil {
		t.Fatalf("生成身份失败: %v", err)
	}

	if err := Save(id1, keyPath); err != nil {
		t.Fatalf("保存身份失败: %v", err)
	}

	// 加载并验证
	id2, err := Load(keyPath)
	if err != nil {
		t.Fatalf("加载身份失败: %v", err)
	}

	if id1.PublicKeyBase64() != id2.PublicKeyBase64() {
		t.Error("加载的公钥与原始不匹配")
	}
}

func TestLoadOrCreate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "identity-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	keyPath := filepath.Join(tmpDir, "identity.key")

	// 首次调用应该创建新身份
	id1, err := LoadOrCreate(keyPath)
	if err != nil {
		t.Fatalf("LoadOrCreate 失败: %v", err)
	}

	// 再次调用应该加载相同身份
	id2, err := LoadOrCreate(keyPath)
	if err != nil {
		t.Fatalf("LoadOrCreate 失败: %v", err)
	}

	if id1.PublicKeyBase64() != id2.PublicKeyBase64() {
		t.Error("两次 LoadOrCreate 返回不同的身份")
	}
}
```

**Step 3: 运行测试验证失败**

```bash
go test ./internal/agent/identity/... -v
```

Expected: FAIL - Generate, Save, Load, LoadOrCreate 函数未定义

**Step 4: 实现 identity.go**

```go
package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
)

// Generate 生成新的 Ed25519 密钥对
func Generate() (*Identity, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成密钥对失败: %w", err)
	}

	return &Identity{
		PrivateKey: priv,
		PublicKey:  pub,
	}, nil
}

// Save 将身份保存到文件（PEM 格式）
func Save(id *Identity, path string) error {
	block := &pem.Block{
		Type:  "ED25519 PRIVATE KEY",
		Bytes: id.PrivateKey,
	}

	data := pem.EncodeToMemory(block)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("写入密钥文件失败: %w", err)
	}

	return nil
}

// Load 从文件加载身份
func Load(path string) (*Identity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取密钥文件失败: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("无效的 PEM 数据")
	}

	if block.Type != "ED25519 PRIVATE KEY" {
		return nil, fmt.Errorf("不支持的密钥类型: %s", block.Type)
	}

	priv := ed25519.PrivateKey(block.Bytes)
	pub := priv.Public().(ed25519.PublicKey)

	return &Identity{
		PrivateKey: priv,
		PublicKey:  pub,
	}, nil
}

// LoadOrCreate 加载现有身份，如果不存在则创建新的
func LoadOrCreate(path string) (*Identity, error) {
	// 尝试加载
	if _, err := os.Stat(path); err == nil {
		return Load(path)
	}

	// 创建新身份
	id, err := Generate()
	if err != nil {
		return nil, err
	}

	// 确保目录存在
	dir := path[:len(path)-len("/identity.key")]
	if dir != path {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("创建目录失败: %w", err)
		}
	}

	// 保存
	if err := Save(id, path); err != nil {
		return nil, err
	}

	return id, nil
}
```

**Step 5: 运行测试验证通过**

```bash
go test ./internal/agent/identity/... -v
```

Expected: PASS

**Step 6: 提交**

```bash
git add -A
git commit -m "feat(identity): 实现 Ed25519 密钥对生成和持久化

- Generate: 生成新的 Ed25519 密钥对
- Save/Load: PEM 格式持久化
- LoadOrCreate: 自动创建或加载已有身份

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

### Task 1.2: 设备指纹采集

**Files:**
- Create: `internal/agent/identity/fingerprint.go`
- Create: `internal/agent/identity/fingerprint_test.go`

**Step 1: 编写失败测试 internal/agent/identity/fingerprint_test.go**

```go
package identity

import (
	"regexp"
	"testing"
)

func TestGenerateFingerprint(t *testing.T) {
	fp, err := GenerateFingerprint()
	if err != nil {
		t.Fatalf("生成指纹失败: %v", err)
	}

	// 指纹应该是 64 字符的十六进制字符串（SHA256）
	if len(fp) != 64 {
		t.Errorf("指纹长度错误: got %d, want 64", len(fp))
	}

	// 验证是有效的十六进制
	matched, _ := regexp.MatchString("^[a-f0-9]{64}$", fp)
	if !matched {
		t.Errorf("指纹格式无效: %s", fp)
	}

	// 多次调用应该返回相同结果
	fp2, _ := GenerateFingerprint()
	if fp != fp2 {
		t.Error("指纹不稳定，多次调用返回不同结果")
	}
}
```

**Step 2: 运行测试验证失败**

```bash
go test ./internal/agent/identity/... -v -run TestGenerateFingerprint
```

Expected: FAIL - GenerateFingerprint 未定义

**Step 3: 实现 fingerprint.go**

```go
package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"runtime"
	"sort"
	"strings"
)

// GenerateFingerprint 生成设备指纹
// 基于主机名、操作系统、架构和网卡 MAC 地址
func GenerateFingerprint() (string, error) {
	var parts []string

	// 主机名
	hostname, err := os.Hostname()
	if err == nil {
		parts = append(parts, "hostname:"+hostname)
	}

	// 操作系统和架构
	parts = append(parts, "os:"+runtime.GOOS)
	parts = append(parts, "arch:"+runtime.GOARCH)

	// 网卡 MAC 地址
	macs, err := getMACAddresses()
	if err == nil && len(macs) > 0 {
		// 排序以确保稳定性
		sort.Strings(macs)
		parts = append(parts, "macs:"+strings.Join(macs, ","))
	}

	// 如果没有收集到任何信息，返回错误
	if len(parts) == 0 {
		return "", fmt.Errorf("无法收集设备信息")
	}

	// 计算 SHA256 哈希
	data := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:]), nil
}

// getMACAddresses 获取所有物理网卡的 MAC 地址
func getMACAddresses() ([]string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var macs []string
	for _, iface := range interfaces {
		// 跳过回环接口和无 MAC 的接口
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if len(iface.HardwareAddr) == 0 {
			continue
		}
		// 跳过虚拟接口（通常以特定前缀开头）
		mac := iface.HardwareAddr.String()
		if strings.HasPrefix(mac, "00:00:00") {
			continue
		}
		macs = append(macs, mac)
	}

	return macs, nil
}
```

**Step 4: 运行测试验证通过**

```bash
go test ./internal/agent/identity/... -v -run TestGenerateFingerprint
```

Expected: PASS

**Step 5: 提交**

```bash
git add -A
git commit -m "feat(identity): 实现设备指纹采集

- 基于主机名、OS、架构、MAC 地址生成 SHA256 指纹
- 排序 MAC 地址确保稳定性
- 过滤回环和虚拟接口

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 2: NAT 探测模块

### Task 2.1: NAT 类型定义

**Files:**
- Create: `internal/agent/nat/types.go`

**Step 1: 创建类型定义 internal/agent/nat/types.go**

```go
package nat

import "fmt"

// NATType 表示 NAT 类型
type NATType int

const (
	NATTypeUnknown        NATType = iota // 未知
	NATTypeNone                          // 公网直连，无 NAT
	NATTypeFullCone                      // 完全锥形 NAT
	NATTypeRestricted                    // 受限锥形 NAT
	NATTypePortRestricted                // 端口受限锥形 NAT
	NATTypeSymmetric                     // 对称型 NAT
)

// String 返回 NAT 类型的字符串表示
func (t NATType) String() string {
	switch t {
	case NATTypeNone:
		return "None"
	case NATTypeFullCone:
		return "FullCone"
	case NATTypeRestricted:
		return "Restricted"
	case NATTypePortRestricted:
		return "PortRestricted"
	case NATTypeSymmetric:
		return "Symmetric"
	default:
		return "Unknown"
	}
}

// CanPunchThrough 判断两个 NAT 类型是否可能打洞成功
func CanPunchThrough(a, b NATType) bool {
	// 公网或完全锥形可以与任何类型打洞
	if a == NATTypeNone || a == NATTypeFullCone {
		return true
	}
	if b == NATTypeNone || b == NATTypeFullCone {
		return true
	}
	// 两个对称型无法打洞
	if a == NATTypeSymmetric && b == NATTypeSymmetric {
		return false
	}
	// 受限锥形之间可以打洞
	if (a == NATTypeRestricted || a == NATTypePortRestricted) &&
		(b == NATTypeRestricted || b == NATTypePortRestricted) {
		return true
	}
	// 对称型与锥形尝试打洞（可能成功）
	return true
}

// Result 表示 NAT 探测结果
type Result struct {
	PublicAddr string  // 公网地址 IP:Port
	NATType    NATType // NAT 类型
	LocalAddr  string  // 本地地址
}

func (r *Result) String() string {
	return fmt.Sprintf("NAT{type=%s, public=%s, local=%s}", r.NATType, r.PublicAddr, r.LocalAddr)
}
```

**Step 2: 验证编译通过**

```bash
go build ./internal/agent/nat/...
```

Expected: 编译成功

**Step 3: 提交**

```bash
git add -A
git commit -m "feat(nat): 定义 NAT 类型和探测结果结构

- NATType 枚举（None/FullCone/Restricted/PortRestricted/Symmetric）
- CanPunchThrough 判断打洞可能性
- Result 结构存储探测结果

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

### Task 2.2: STUN 探测实现

**Files:**
- Create: `internal/agent/nat/prober.go`
- Create: `internal/agent/nat/prober_test.go`

**Step 1: 添加依赖**

```bash
go get github.com/pion/stun
```

**Step 2: 编写失败测试 internal/agent/nat/prober_test.go**

```go
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
```

**Step 3: 运行测试验证失败**

```bash
go test ./internal/agent/nat/... -v
```

Expected: FAIL - NewProber 未定义

**Step 4: 实现 prober.go**

```go
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
```

**Step 5: 运行测试验证通过**

```bash
go test ./internal/agent/nat/... -v
```

Expected: PASS（需要网络连接）

**Step 6: 提交**

```bash
git add -A
git commit -m "feat(nat): 实现 STUN 探测器

- 支持多 STUN 服务器故障转移
- 解析 XOR-MAPPED-ADDRESS 和 MAPPED-ADDRESS
- 基础 NAT 类型检测

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 3: 路由管理模块

### Task 3.1: 路由管理接口和实现

**Files:**
- Create: `internal/agent/route/manager.go`
- Create: `internal/agent/route/manager_linux.go`
- Create: `internal/agent/route/manager_stub.go`

**Step 1: 添加依赖**

```bash
go get github.com/vishvananda/netlink
```

**Step 2: 创建接口定义 internal/agent/route/manager.go**

```go
package route

import "net"

// Manager 定义路由管理接口
type Manager interface {
	// AddRoute 添加路由
	AddRoute(dst *net.IPNet, gw net.IP, ifaceName string) error

	// RemoveRoute 删除路由
	RemoveRoute(dst *net.IPNet) error

	// GetRoutes 获取指定接口的路由
	GetRoutes(ifaceName string) ([]Route, error)
}

// Route 表示一条路由
type Route struct {
	Dst       *net.IPNet // 目标网络
	Gw        net.IP     // 网关
	IfaceName string     // 出接口名称
}

// New 创建路由管理器
func New() Manager {
	return newPlatformManager()
}
```

**Step 3: 创建 Linux 实现 internal/agent/route/manager_linux.go**

```go
//go:build linux

package route

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

type linuxManager struct{}

func newPlatformManager() Manager {
	return &linuxManager{}
}

func (m *linuxManager) AddRoute(dst *net.IPNet, gw net.IP, ifaceName string) error {
	// 获取接口
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("获取接口 %s 失败: %w", ifaceName, err)
	}

	route := &netlink.Route{
		Dst:       dst,
		Gw:        gw,
		LinkIndex: link.Attrs().Index,
	}

	if err := netlink.RouteAdd(route); err != nil {
		return fmt.Errorf("添加路由失败: %w", err)
	}

	return nil
}

func (m *linuxManager) RemoveRoute(dst *net.IPNet) error {
	route := &netlink.Route{
		Dst: dst,
	}

	if err := netlink.RouteDel(route); err != nil {
		return fmt.Errorf("删除路由失败: %w", err)
	}

	return nil
}

func (m *linuxManager) GetRoutes(ifaceName string) ([]Route, error) {
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return nil, fmt.Errorf("获取接口 %s 失败: %w", ifaceName, err)
	}

	routes, err := netlink.RouteList(link, netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("获取路由列表失败: %w", err)
	}

	var result []Route
	for _, r := range routes {
		result = append(result, Route{
			Dst:       r.Dst,
			Gw:        r.Gw,
			IfaceName: ifaceName,
		})
	}

	return result, nil
}
```

**Step 4: 创建桩实现（非 Linux）internal/agent/route/manager_stub.go**

```go
//go:build !linux

package route

import (
	"fmt"
	"net"
	"runtime"
)

type stubManager struct{}

func newPlatformManager() Manager {
	return &stubManager{}
}

func (m *stubManager) AddRoute(dst *net.IPNet, gw net.IP, ifaceName string) error {
	return fmt.Errorf("路由管理在 %s 平台上未实现", runtime.GOOS)
}

func (m *stubManager) RemoveRoute(dst *net.IPNet) error {
	return fmt.Errorf("路由管理在 %s 平台上未实现", runtime.GOOS)
}

func (m *stubManager) GetRoutes(ifaceName string) ([]Route, error) {
	return nil, fmt.Errorf("路由管理在 %s 平台上未实现", runtime.GOOS)
}
```

**Step 5: 验证编译通过**

```bash
go build ./internal/agent/route/...
```

Expected: 编译成功

**Step 6: 提交**

```bash
git add -A
git commit -m "feat(route): 实现路由管理模块

- 定义 Manager 接口
- Linux: 使用 netlink 管理路由
- 其他平台: 桩实现

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 4: gRPC Proto 定义

### Task 4.1: 安装 protoc 工具

**Step 1: 安装 protoc-gen-go 插件**

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

**Step 2: 添加 gRPC 依赖**

```bash
go get google.golang.org/grpc
go get google.golang.org/protobuf
```

---

### Task 4.2: Management Proto

**Files:**
- Create: `pkg/proto/management/management.proto`
- Create: `pkg/proto/management/generate.go`

**Step 1: 创建 Management Proto 文件 pkg/proto/management/management.proto**

```protobuf
syntax = "proto3";

package govpc.management.v1;

option go_package = "github.com/example/go-vpc/pkg/proto/management";

// ManagementService 管理服务
service ManagementService {
  // Register 注册设备
  rpc Register(RegisterRequest) returns (RegisterResponse);
  // Authenticate 认证设备
  rpc Authenticate(AuthRequest) returns (AuthResponse);
  // Heartbeat 心跳
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
  // GetPeerInfo 获取 Peer 信息
  rpc GetPeerInfo(GetPeerInfoRequest) returns (GetPeerInfoResponse);
}

message RegisterRequest {
  string public_key = 1;
  string device_fingerprint = 2;
  string hostname = 3;
  map<string, string> labels = 4;
}

message RegisterResponse {
  string device_id = 1;
  string vip = 2;
  int32 vip_prefix = 3;
  string auth_token = 4;
}

message AuthRequest {
  string device_id = 1;
  string auth_token = 2;
  string public_key = 3;
}

message AuthResponse {
  bool success = 1;
  Config config = 2;
  string error = 3;
}

message Config {
  repeated RelayServer relays = 1;
  repeated ACLRule acl_rules = 2;
  int64 config_version = 3;
}

message RelayServer {
  string address = 1;
  string region = 2;
  int32 priority = 3;
}

message ACLRule {
  string src_vip = 1;
  string dst_vip = 2;
  string action = 3;
}

message HeartbeatRequest {
  string device_id = 1;
  string auth_token = 2;
  AgentStatus status = 3;
}

message AgentStatus {
  int64 uptime_seconds = 1;
  int32 active_peers = 2;
  int64 bytes_sent = 3;
  int64 bytes_received = 4;
}

message HeartbeatResponse {
  bool config_updated = 1;
  int64 latest_config_version = 2;
}

message GetPeerInfoRequest {
  string vip = 1;
}

message GetPeerInfoResponse {
  string device_id = 1;
  string public_key = 2;
  repeated string allowed_ips = 3;
}
```

**Step 2: 创建生成脚本注释 pkg/proto/management/generate.go**

```go
package management

//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative management.proto
```

**Step 3: 生成 Go 代码**

```bash
cd pkg/proto/management && protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative management.proto && cd ../../..
```

Expected: 生成 management.pb.go 和 management_grpc.pb.go

**Step 4: 提交**

```bash
git add -A
git commit -m "feat(proto): 添加 Management Service Proto 定义

- Register: 设备注册
- Authenticate: 设备认证
- Heartbeat: 心跳上报
- GetPeerInfo: 获取 Peer 信息

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

### Task 4.3: Signal Proto

**Files:**
- Create: `pkg/proto/signal/signal.proto`
- Create: `pkg/proto/signal/generate.go`

**Step 1: 创建 Signal Proto 文件 pkg/proto/signal/signal.proto**

```protobuf
syntax = "proto3";

package govpc.signal.v1;

option go_package = "github.com/example/go-vpc/pkg/proto/signal";

// SignalService 信令服务
service SignalService {
  // Connect 建立双向流连接
  rpc Connect(stream SignalMessage) returns (stream SignalMessage);
}

message SignalMessage {
  string message_id = 1;

  oneof payload {
    RegisterEndpoint register_endpoint = 10;
    HandshakeRequest handshake_request = 11;
    HandshakeResponse handshake_response = 12;
    Ping ping = 13;

    EndpointRegistered endpoint_registered = 20;
    IncomingHandshake incoming_handshake = 21;
    HandshakeResult handshake_result = 22;
    Pong pong = 23;
    PeerEndpointUpdate peer_endpoint_update = 24;
  }
}

message RegisterEndpoint {
  string device_id = 1;
  string auth_token = 2;
  Endpoint endpoint = 3;
}

message Endpoint {
  string public_addr = 1;
  NATType nat_type = 2;
  int64 timestamp = 3;
}

enum NATType {
  NAT_TYPE_UNKNOWN = 0;
  NAT_TYPE_NONE = 1;
  NAT_TYPE_FULL_CONE = 2;
  NAT_TYPE_RESTRICTED = 3;
  NAT_TYPE_PORT_RESTRICTED = 4;
  NAT_TYPE_SYMMETRIC = 5;
}

message EndpointRegistered {
  bool success = 1;
  string error = 2;
}

message HandshakeRequest {
  string target_vip = 1;
  string wg_public_key = 2;
  Endpoint my_endpoint = 3;
}

message IncomingHandshake {
  string from_device_id = 1;
  string from_vip = 2;
  string wg_public_key = 3;
  Endpoint peer_endpoint = 4;
}

message HandshakeResponse {
  string target_device_id = 1;
  bool accept = 2;
  string wg_public_key = 3;
  Endpoint my_endpoint = 4;
}

message HandshakeResult {
  string peer_device_id = 1;
  string peer_vip = 2;
  bool success = 3;
  string wg_public_key = 4;
  Endpoint peer_endpoint = 5;
  string error = 6;
}

message PeerEndpointUpdate {
  string peer_device_id = 1;
  string peer_vip = 2;
  Endpoint new_endpoint = 3;
}

message Ping {
  int64 timestamp = 1;
}

message Pong {
  int64 timestamp = 1;
}
```

**Step 2: 创建生成脚本注释 pkg/proto/signal/generate.go**

```go
package signal

//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative signal.proto
```

**Step 3: 生成 Go 代码**

```bash
cd pkg/proto/signal && protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative signal.proto && cd ../../..
```

Expected: 生成 signal.pb.go 和 signal_grpc.pb.go

**Step 4: 提交**

```bash
git add -A
git commit -m "feat(proto): 添加 Signal Service Proto 定义

- Connect: 双向流连接
- RegisterEndpoint: 端点注册
- HandshakeRequest/Response: 握手协商
- PeerEndpointUpdate: Peer 端点更新

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 5: gRPC 客户端

### Task 5.1: Management 客户端

**Files:**
- Create: `internal/agent/client/management.go`
- Create: `internal/agent/client/management_test.go`

**Step 1: 创建 Management 客户端 internal/agent/client/management.go**

```go
package client

import (
	"context"
	"fmt"
	"time"

	pb "github.com/example/go-vpc/pkg/proto/management"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ManagementClient 管理服务客户端
type ManagementClient struct {
	conn   *grpc.ClientConn
	client pb.ManagementServiceClient
}

// NewManagementClient 创建管理服务客户端
func NewManagementClient(addr string) (*ManagementClient, error) {
	// TODO: 生产环境使用 TLS
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("连接 Management Server 失败: %w", err)
	}

	return &ManagementClient{
		conn:   conn,
		client: pb.NewManagementServiceClient(conn),
	}, nil
}

// Close 关闭连接
func (c *ManagementClient) Close() error {
	return c.conn.Close()
}

// Register 注册设备
func (c *ManagementClient) Register(ctx context.Context, publicKey, fingerprint, hostname string, labels map[string]string) (*pb.RegisterResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := c.client.Register(ctx, &pb.RegisterRequest{
		PublicKey:         publicKey,
		DeviceFingerprint: fingerprint,
		Hostname:          hostname,
		Labels:            labels,
	})
	if err != nil {
		return nil, fmt.Errorf("注册失败: %w", err)
	}

	return resp, nil
}

// Authenticate 认证设备
func (c *ManagementClient) Authenticate(ctx context.Context, deviceID, authToken, publicKey string) (*pb.AuthResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := c.client.Authenticate(ctx, &pb.AuthRequest{
		DeviceId:  deviceID,
		AuthToken: authToken,
		PublicKey: publicKey,
	})
	if err != nil {
		return nil, fmt.Errorf("认证失败: %w", err)
	}

	return resp, nil
}

// Heartbeat 发送心跳
func (c *ManagementClient) Heartbeat(ctx context.Context, deviceID, authToken string, status *pb.AgentStatus) (*pb.HeartbeatResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.client.Heartbeat(ctx, &pb.HeartbeatRequest{
		DeviceId:  deviceID,
		AuthToken: authToken,
		Status:    status,
	})
	if err != nil {
		return nil, fmt.Errorf("心跳失败: %w", err)
	}

	return resp, nil
}

// GetPeerInfo 获取 Peer 信息
func (c *ManagementClient) GetPeerInfo(ctx context.Context, vip string) (*pb.GetPeerInfoResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.client.GetPeerInfo(ctx, &pb.GetPeerInfoRequest{
		Vip: vip,
	})
	if err != nil {
		return nil, fmt.Errorf("获取 Peer 信息失败: %w", err)
	}

	return resp, nil
}
```

**Step 2: 验证编译通过**

```bash
go build ./internal/agent/client/...
```

Expected: 编译成功

**Step 3: 提交**

```bash
git add -A
git commit -m "feat(client): 实现 Management gRPC 客户端

- Register: 设备注册
- Authenticate: 设备认证
- Heartbeat: 心跳上报
- GetPeerInfo: 获取 Peer 信息

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

### Task 5.2: Signal 客户端

**Files:**
- Create: `internal/agent/client/signal.go`

**Step 1: 创建 Signal 客户端 internal/agent/client/signal.go**

```go
package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	pb "github.com/example/go-vpc/pkg/proto/signal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// SignalClient Signal 服务客户端
type SignalClient struct {
	conn   *grpc.ClientConn
	client pb.SignalServiceClient
	stream pb.SignalService_ConnectClient

	mu        sync.Mutex
	handlers  map[string]func(*pb.SignalMessage)
	onMessage func(*pb.SignalMessage)
}

// NewSignalClient 创建 Signal 服务客户端
func NewSignalClient(addr string) (*SignalClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("连接 Signal Server 失败: %w", err)
	}

	return &SignalClient{
		conn:     conn,
		client:   pb.NewSignalServiceClient(conn),
		handlers: make(map[string]func(*pb.SignalMessage)),
	}, nil
}

// Connect 建立双向流连接
func (c *SignalClient) Connect(ctx context.Context) error {
	stream, err := c.client.Connect(ctx)
	if err != nil {
		return fmt.Errorf("建立流连接失败: %w", err)
	}

	c.stream = stream

	// 启动接收协程
	go c.receiveLoop()

	return nil
}

// Close 关闭连接
func (c *SignalClient) Close() error {
	if c.stream != nil {
		c.stream.CloseSend()
	}
	return c.conn.Close()
}

// SetMessageHandler 设置消息处理回调
func (c *SignalClient) SetMessageHandler(handler func(*pb.SignalMessage)) {
	c.onMessage = handler
}

// Send 发送消息
func (c *SignalClient) Send(msg *pb.SignalMessage) error {
	if c.stream == nil {
		return fmt.Errorf("流连接未建立")
	}

	return c.stream.Send(msg)
}

// RegisterEndpoint 注册端点
func (c *SignalClient) RegisterEndpoint(deviceID, authToken string, endpoint *pb.Endpoint) error {
	return c.Send(&pb.SignalMessage{
		MessageId: fmt.Sprintf("reg-%d", time.Now().UnixNano()),
		Payload: &pb.SignalMessage_RegisterEndpoint{
			RegisterEndpoint: &pb.RegisterEndpoint{
				DeviceId:  deviceID,
				AuthToken: authToken,
				Endpoint:  endpoint,
			},
		},
	})
}

// SendHandshakeRequest 发送握手请求
func (c *SignalClient) SendHandshakeRequest(targetVIP, wgPublicKey string, myEndpoint *pb.Endpoint) error {
	return c.Send(&pb.SignalMessage{
		MessageId: fmt.Sprintf("hs-%d", time.Now().UnixNano()),
		Payload: &pb.SignalMessage_HandshakeRequest{
			HandshakeRequest: &pb.HandshakeRequest{
				TargetVip:   targetVIP,
				WgPublicKey: wgPublicKey,
				MyEndpoint:  myEndpoint,
			},
		},
	})
}

// SendHandshakeResponse 发送握手响应
func (c *SignalClient) SendHandshakeResponse(targetDeviceID string, accept bool, wgPublicKey string, myEndpoint *pb.Endpoint) error {
	return c.Send(&pb.SignalMessage{
		MessageId: fmt.Sprintf("hsr-%d", time.Now().UnixNano()),
		Payload: &pb.SignalMessage_HandshakeResponse{
			HandshakeResponse: &pb.HandshakeResponse{
				TargetDeviceId: targetDeviceID,
				Accept:         accept,
				WgPublicKey:    wgPublicKey,
				MyEndpoint:     myEndpoint,
			},
		},
	})
}

// SendPing 发送心跳
func (c *SignalClient) SendPing() error {
	return c.Send(&pb.SignalMessage{
		MessageId: fmt.Sprintf("ping-%d", time.Now().UnixNano()),
		Payload: &pb.SignalMessage_Ping{
			Ping: &pb.Ping{
				Timestamp: time.Now().UnixMilli(),
			},
		},
	})
}

// receiveLoop 接收消息循环
func (c *SignalClient) receiveLoop() {
	for {
		msg, err := c.stream.Recv()
		if err != nil {
			// 连接断开
			return
		}

		if c.onMessage != nil {
			c.onMessage(msg)
		}
	}
}
```

**Step 2: 验证编译通过**

```bash
go build ./internal/agent/client/...
```

Expected: 编译成功

**Step 3: 提交**

```bash
git add -A
git commit -m "feat(client): 实现 Signal gRPC 客户端

- Connect: 建立双向流连接
- RegisterEndpoint: 注册端点
- SendHandshakeRequest/Response: 握手协商
- SendPing: 心跳保活

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 6: WireGuard 引擎

### Task 6.1: WireGuard 类型定义

**Files:**
- Create: `internal/agent/wireguard/types.go`

**Step 1: 创建类型定义 internal/agent/wireguard/types.go**

```go
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
```

**Step 2: 验证编译通过**

```bash
go build ./internal/agent/wireguard/...
```

Expected: 编译成功

**Step 3: 提交**

```bash
git add -A
git commit -m "feat(wireguard): 定义 WireGuard 引擎接口

- Config: WireGuard 配置
- PeerConfig: Peer 配置
- Engine: 引擎接口定义

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

### Task 6.2: wireguard-go 集成

**Files:**
- Create: `internal/agent/wireguard/userspace.go`

**Step 1: 添加依赖**

```bash
go get golang.zx2c4.com/wireguard
go get golang.zx2c4.com/wireguard/wgctrl
go get golang.zx2c4.com/wireguard/wgctrl/wgtypes
```

**Step 2: 创建 userspace 实现 internal/agent/wireguard/userspace.go**

```go
package wireguard

import (
	"fmt"
	"net"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// UserspaceEngine 基于 wireguard-go 的引擎实现
type UserspaceEngine struct {
	ifaceName  string
	listenPort int
	client     *wgctrl.Client
}

// NewUserspaceEngine 创建 userspace 引擎
func NewUserspaceEngine(ifaceName string) (*UserspaceEngine, error) {
	client, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("创建 wgctrl 客户端失败: %w", err)
	}

	return &UserspaceEngine{
		ifaceName: ifaceName,
		client:    client,
	}, nil
}

// Init 初始化引擎
func (e *UserspaceEngine) Init(cfg Config, localVIP net.IP, vipPrefix int) error {
	// 解析私钥
	privKey, err := wgtypes.ParseKey(cfg.PrivateKey)
	if err != nil {
		return fmt.Errorf("解析私钥失败: %w", err)
	}

	port := cfg.ListenPort

	// 配置 WireGuard 设备
	wgConfig := wgtypes.Config{
		PrivateKey: &privKey,
		ListenPort: &port,
	}

	if err := e.client.ConfigureDevice(e.ifaceName, wgConfig); err != nil {
		return fmt.Errorf("配置 WireGuard 设备失败: %w", err)
	}

	e.listenPort = port
	return nil
}

// AddPeer 添加 Peer
func (e *UserspaceEngine) AddPeer(peer PeerConfig) error {
	pubKey, err := wgtypes.ParseKey(peer.PublicKey)
	if err != nil {
		return fmt.Errorf("解析公钥失败: %w", err)
	}

	var endpoint *net.UDPAddr
	if peer.Endpoint != "" {
		endpoint, err = net.ResolveUDPAddr("udp", peer.Endpoint)
		if err != nil {
			return fmt.Errorf("解析端点地址失败: %w", err)
		}
	}

	var allowedIPs []net.IPNet
	for _, cidr := range peer.AllowedIPs {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			return fmt.Errorf("解析 AllowedIP %s 失败: %w", cidr, err)
		}
		allowedIPs = append(allowedIPs, *ipnet)
	}

	keepalive := 25 * time.Second
	peerConfig := wgtypes.PeerConfig{
		PublicKey:                   pubKey,
		Endpoint:                    endpoint,
		AllowedIPs:                  allowedIPs,
		PersistentKeepaliveInterval: &keepalive,
	}

	wgConfig := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{peerConfig},
	}

	if err := e.client.ConfigureDevice(e.ifaceName, wgConfig); err != nil {
		return fmt.Errorf("添加 Peer 失败: %w", err)
	}

	return nil
}

// RemovePeer 移除 Peer
func (e *UserspaceEngine) RemovePeer(publicKey string) error {
	pubKey, err := wgtypes.ParseKey(publicKey)
	if err != nil {
		return fmt.Errorf("解析公钥失败: %w", err)
	}

	peerConfig := wgtypes.PeerConfig{
		PublicKey: pubKey,
		Remove:    true,
	}

	wgConfig := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{peerConfig},
	}

	if err := e.client.ConfigureDevice(e.ifaceName, wgConfig); err != nil {
		return fmt.Errorf("移除 Peer 失败: %w", err)
	}

	return nil
}

// UpdatePeerEndpoint 更新 Peer 端点
func (e *UserspaceEngine) UpdatePeerEndpoint(publicKey string, endpoint string) error {
	pubKey, err := wgtypes.ParseKey(publicKey)
	if err != nil {
		return fmt.Errorf("解析公钥失败: %w", err)
	}

	udpAddr, err := net.ResolveUDPAddr("udp", endpoint)
	if err != nil {
		return fmt.Errorf("解析端点地址失败: %w", err)
	}

	peerConfig := wgtypes.PeerConfig{
		PublicKey:         pubKey,
		Endpoint:          udpAddr,
		UpdateOnly:        true,
		ReplaceAllowedIPs: false,
	}

	wgConfig := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{peerConfig},
	}

	if err := e.client.ConfigureDevice(e.ifaceName, wgConfig); err != nil {
		return fmt.Errorf("更新 Peer 端点失败: %w", err)
	}

	return nil
}

// GetListenPort 获取监听端口
func (e *UserspaceEngine) GetListenPort() int {
	return e.listenPort
}

// Close 关闭引擎
func (e *UserspaceEngine) Close() error {
	return e.client.Close()
}
```

**Step 3: 验证编译通过**

```bash
go build ./internal/agent/wireguard/...
```

Expected: 编译成功

**Step 4: 提交**

```bash
git add -A
git commit -m "feat(wireguard): 实现 wireguard-go userspace 引擎

- 使用 wgctrl 库管理 WireGuard 设备
- 支持 Peer 添加/移除/更新端点
- 25 秒 keepalive 间隔

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 7: 配置模块

### Task 7.1: 配置结构和加载

**Files:**
- Create: `internal/agent/config/config.go`
- Create: `configs/agent.example.yaml`

**Step 1: 添加依赖**

```bash
go get gopkg.in/yaml.v3
```

**Step 2: 创建配置结构 internal/agent/config/config.go**

```go
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config Agent 配置
type Config struct {
	Agent      AgentConfig      `yaml:"agent"`
	Server     ServerConfig     `yaml:"server"`
	WireGuard  WireGuardConfig  `yaml:"wireguard"`
	NAT        NATConfig        `yaml:"nat"`
	Network    NetworkConfig    `yaml:"network"`
	Connection ConnectionConfig `yaml:"connection"`
}

// AgentConfig Agent 基础配置
type AgentConfig struct {
	DataDir  string `yaml:"data_dir"`
	LogLevel string `yaml:"log_level"`
}

// ServerConfig 服务端配置
type ServerConfig struct {
	Management ManagementConfig `yaml:"management"`
	Signal     []SignalConfig   `yaml:"signal"`
}

// ManagementConfig Management 服务配置
type ManagementConfig struct {
	Address string `yaml:"address"`
	TLS     bool   `yaml:"tls"`
}

// SignalConfig Signal 服务配置
type SignalConfig struct {
	Address string `yaml:"address"`
	Region  string `yaml:"region"`
}

// WireGuardConfig WireGuard 配置
type WireGuardConfig struct {
	ListenPort     int  `yaml:"listen_port"`
	ForceUserspace bool `yaml:"force_userspace"`
	MTU            int  `yaml:"mtu"`
}

// NATConfig NAT 配置
type NATConfig struct {
	STUNServers []string      `yaml:"stun_servers"`
	Timeout     time.Duration `yaml:"timeout"`
}

// NetworkConfig 网络监控配置
type NetworkConfig struct {
	PollInterval time.Duration `yaml:"poll_interval"`
}

// ConnectionConfig 连接配置
type ConnectionConfig struct {
	HandshakeTimeout    time.Duration `yaml:"handshake_timeout"`
	HeartbeatInterval   time.Duration `yaml:"heartbeat_interval"`
	MaxReconnectBackoff time.Duration `yaml:"max_reconnect_backoff"`
}

// Load 从文件加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 设置默认值
	cfg.setDefaults()

	return cfg, nil
}

// setDefaults 设置默认值
func (c *Config) setDefaults() {
	if c.Agent.DataDir == "" {
		c.Agent.DataDir = "/var/lib/go-vpc"
	}
	if c.Agent.LogLevel == "" {
		c.Agent.LogLevel = "info"
	}
	if c.WireGuard.MTU == 0 {
		c.WireGuard.MTU = 1420
	}
	if c.NAT.Timeout == 0 {
		c.NAT.Timeout = 3 * time.Second
	}
	if len(c.NAT.STUNServers) == 0 {
		c.NAT.STUNServers = []string{
			"stun.l.google.com:19302",
			"stun.cloudflare.com:3478",
		}
	}
	if c.Network.PollInterval == 0 {
		c.Network.PollInterval = 30 * time.Second
	}
	if c.Connection.HandshakeTimeout == 0 {
		c.Connection.HandshakeTimeout = 3 * time.Second
	}
	if c.Connection.HeartbeatInterval == 0 {
		c.Connection.HeartbeatInterval = 15 * time.Second
	}
	if c.Connection.MaxReconnectBackoff == 0 {
		c.Connection.MaxReconnectBackoff = 60 * time.Second
	}
}

// Default 返回默认配置
func Default() *Config {
	cfg := &Config{}
	cfg.setDefaults()
	return cfg
}
```

**Step 3: 创建示例配置 configs/agent.example.yaml**

```yaml
# Go-VPC Agent 配置文件

agent:
  # 数据目录（存放密钥、状态等）
  data_dir: /var/lib/go-vpc
  # 日志级别: debug, info, warn, error
  log_level: info

server:
  management:
    address: management.example.com:443
    tls: true
  signal:
    - address: signal-cn.example.com:443
      region: cn
    - address: signal-us.example.com:443
      region: us

wireguard:
  # 监听端口（0 表示随机）
  listen_port: 51820
  # 强制使用 userspace 实现
  force_userspace: false
  # MTU
  mtu: 1420

nat:
  stun_servers:
    - stun.l.google.com:19302
    - stun.cloudflare.com:3478
  timeout: 3s

network:
  poll_interval: 30s

connection:
  handshake_timeout: 3s
  heartbeat_interval: 15s
  max_reconnect_backoff: 60s
```

**Step 4: 验证编译通过**

```bash
go build ./internal/agent/config/...
```

Expected: 编译成功

**Step 5: 提交**

```bash
git add -A
git commit -m "feat(config): 实现配置加载模块

- YAML 格式配置文件
- 支持默认值
- 包含示例配置文件

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 8: 整合 Agent 主逻辑

### Task 8.1: Agent 完整启动流程

**Files:**
- Modify: `internal/agent/agent.go`

**Step 1: 更新 Agent 主结构 internal/agent/agent.go**

```go
package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/example/go-vpc/internal/agent/config"
	"github.com/example/go-vpc/internal/agent/identity"
	"github.com/example/go-vpc/internal/agent/nat"
)

// Agent 是客户端代理的主结构
type Agent struct {
	cfg      *config.Config
	identity *identity.Identity
	prober   *nat.Prober

	deviceID  string
	authToken string
	vip       string
}

// New 创建新的 Agent 实例
func New(cfg *config.Config) (*Agent, error) {
	if cfg == nil {
		cfg = config.Default()
	}

	return &Agent{
		cfg: cfg,
	}, nil
}

// Start 启动 Agent
func (a *Agent) Start(ctx context.Context) error {
	fmt.Println("Agent 启动中...")

	// 1. 加载或创建身份
	if err := a.loadIdentity(); err != nil {
		return fmt.Errorf("加载身份失败: %w", err)
	}
	fmt.Printf("身份已加载，公钥: %s...\n", a.identity.PublicKeyBase64()[:16])

	// 2. 初始化 NAT 探测器
	a.prober = nat.NewProber(a.cfg.NAT.STUNServers, a.cfg.NAT.Timeout)

	// 3. 执行 NAT 探测
	natResult, err := a.prober.Probe(ctx)
	if err != nil {
		fmt.Printf("警告: NAT 探测失败: %v\n", err)
	} else {
		fmt.Printf("NAT 探测完成: %s\n", natResult)
	}

	// 4. TODO: 连接 Management Server
	// 5. TODO: 初始化 WireGuard
	// 6. TODO: 连接 Signal Server

	fmt.Println("Agent 已启动，等待停止信号...")

	// 启动心跳
	go a.heartbeatLoop(ctx)

	// 等待上下文取消
	<-ctx.Done()
	fmt.Println("Agent 已停止")
	return nil
}

// Stop 停止 Agent
func (a *Agent) Stop() error {
	return nil
}

// loadIdentity 加载或创建身份
func (a *Agent) loadIdentity() error {
	// 确保数据目录存在
	if err := os.MkdirAll(a.cfg.Agent.DataDir, 0700); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	keyPath := filepath.Join(a.cfg.Agent.DataDir, "identity.key")
	id, err := identity.LoadOrCreate(keyPath)
	if err != nil {
		return err
	}

	a.identity = id
	return nil
}

// heartbeatLoop 心跳循环
func (a *Agent) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(a.cfg.Connection.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// TODO: 发送心跳到 Management 和 Signal Server
		}
	}
}
```

**Step 2: 更新入口文件 cmd/agent/main.go**

```go
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
	var cfg *config.Config
	var err error
	if *configPath != "" {
		cfg, err = config.Load(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
			os.Exit(1)
		}
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
```

**Step 3: 验证编译和运行**

```bash
go build ./cmd/agent && ./agent
```

Expected: Agent 启动，执行 NAT 探测，等待停止信号

**Step 4: 运行测试**

```bash
go test ./... -v
```

Expected: 所有测试通过

**Step 5: 提交**

```bash
git add -A
git commit -m "feat(agent): 整合完整启动流程

- 加载/创建身份
- NAT 探测
- 心跳循环
- 命令行参数支持

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## 实现总结

完成以上任务后，Agent P0 阶段将具备：

| 模块 | 状态 | 功能 |
|------|------|------|
| identity | ✅ | Ed25519 密钥对 + 设备指纹 |
| nat | ✅ | STUN 探测 + NAT 类型识别 |
| route | ✅ | Linux 路由表管理 |
| wireguard | ✅ | wireguard-go 集成 |
| client | ✅ | Management + Signal gRPC 客户端 |
| config | ✅ | YAML 配置加载 |
| agent | ✅ | 启动流程整合 |

**下一步（P1）：**
- 网络变化监听与漫游
- 完整的服务端连接流程
- Peer 握手与隧道建立
